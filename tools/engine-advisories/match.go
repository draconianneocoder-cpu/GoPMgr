// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"strconv"
	"strings"
)

// version is a dotted numeric version. Missing parts compare as zero, so
// 3.54 equals 3.54.0 and is less than 3.54.1; 3.54.1.1 is greater than
// 3.54.1.
type version []int

func parseVersion(s string) (version, error) {
	if s == "" {
		return nil, fmt.Errorf("empty version")
	}
	parts := strings.Split(s, ".")
	v := make(version, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p == "" || (len(p) > 1 && p[0] == '0') {
			return nil, fmt.Errorf("version %q is not dotted decimal numbers", s)
		}
		v[i] = n
	}
	return v, nil
}

func (v version) compare(o version) int {
	for i := 0; i < len(v) || i < len(o); i++ {
		a, b := 0, 0
		if i < len(v) {
			a = v[i]
		}
		if i < len(o) {
			b = o[i]
		}
		if a != b {
			if a < b {
				return -1
			}
			return 1
		}
	}
	return 0
}

// The parts of NVD's CVE API 2.0 response that decide applicability. Only
// these fields are read; the rest of the response is ignored.
type nvdCVE struct {
	ID             string             `json:"id"`
	VulnStatus     string             `json:"vulnStatus"`
	LastModified   string             `json:"lastModified"`
	Configurations []nvdConfiguration `json:"configurations"`
}

type nvdConfiguration struct {
	Operator string    `json:"operator"`
	Negate   bool      `json:"negate"`
	Nodes    []nvdNode `json:"nodes"`
}

type nvdNode struct {
	Operator string        `json:"operator"`
	Negate   bool          `json:"negate"`
	CPEMatch []nvdCPEMatch `json:"cpeMatch"`
}

type nvdCPEMatch struct {
	// Vulnerable is a pointer so that a missing field reads as vulnerable,
	// not as "a platform only".
	Vulnerable            *bool  `json:"vulnerable"`
	Criteria              string `json:"criteria"`
	VersionStartIncluding string `json:"versionStartIncluding"`
	VersionStartExcluding string `json:"versionStartExcluding"`
	VersionEndIncluding   string `json:"versionEndIncluding"`
	VersionEndExcluding   string `json:"versionEndExcluding"`
}

// applies decides whether a CVE NVD returned applies to c, and why. It only
// ever narrows NVD's own match, and when in doubt it says the CVE applies:
//
//   - no configurations, or none that names c's vendor and product (NVD
//     returned it for a reason it does not show);
//   - a negated configuration or node;
//   - a version or bound that does not parse.
//
// Otherwise it applies when some cpeMatch for c's vendor and product is
// vulnerable and covers c's version. Whether that match is AND-ed with a
// platform is ignored: it can only add false positives, never hide a CVE.
// An entry for c that is not vulnerable (c as a platform of another
// product's bug) does not apply.
func applies(cve nvdCVE, c component) (bool, string) {
	ours, err := parseVersion(c.Version)
	if err != nil {
		return true, fmt.Sprintf("cannot compare: %v", err)
	}
	if len(cve.Configurations) == 0 {
		return true, "NVD gives no configurations to decide by"
	}
	named := false
	for _, cfg := range cve.Configurations {
		if cfg.Negate {
			return true, "a configuration is negated"
		}
		for _, node := range cfg.Nodes {
			if node.Negate {
				return true, "a configuration node is negated"
			}
			for _, m := range node.CPEMatch {
				part, vendor, product, exact, err := splitCPE(m.Criteria)
				if err != nil {
					return true, fmt.Sprintf("cannot read criteria %q: %v", m.Criteria, err)
				}
				if part != "a" || !strings.EqualFold(vendor, c.CPEVendor) || !strings.EqualFold(product, c.CPEProduct) {
					continue
				}
				named = true
				if m.Vulnerable != nil && !*m.Vulnerable {
					continue
				}
				covered, why := covers(ours, exact, m)
				if covered {
					return true, why
				}
			}
		}
	}
	if !named {
		return true, "no configuration names " + c.CPEVendor + ":" + c.CPEProduct
	}
	return false, "no vulnerable configuration covers " + c.Version
}

// covers reports whether a vulnerable cpeMatch covers ours: the criteria's
// version is any ("*"), not applicable ("-", read as any) or equal to ours,
// and every bound given holds.
func covers(ours version, exact string, m nvdCPEMatch) (bool, string) {
	switch exact {
	case "*", "-":
	default:
		v, err := parseVersion(exact)
		if err != nil {
			return true, fmt.Sprintf("cannot compare criteria version %q", exact)
		}
		if v.compare(ours) != 0 {
			return false, ""
		}
	}
	bounds := []struct {
		value string
		ok    func(c int) bool
		text  string
	}{
		{m.VersionStartIncluding, func(c int) bool { return c >= 0 }, "from %s"},
		{m.VersionStartExcluding, func(c int) bool { return c > 0 }, "after %s"},
		{m.VersionEndIncluding, func(c int) bool { return c <= 0 }, "through %s"},
		{m.VersionEndExcluding, func(c int) bool { return c < 0 }, "before %s"},
	}
	var described []string
	for _, b := range bounds {
		if b.value == "" {
			continue
		}
		limit, err := parseVersion(b.value)
		if err != nil {
			return true, fmt.Sprintf("cannot compare bound %q", b.value)
		}
		if !b.ok(ours.compare(limit)) {
			return false, ""
		}
		described = append(described, fmt.Sprintf(b.text, b.value))
	}
	if len(described) == 0 {
		return true, "vulnerable: " + m.Criteria
	}
	return true, "vulnerable " + strings.Join(described, ", ")
}

// splitCPE returns the part, vendor, product and version of a CPE 2.3
// formatted string. Backslash escapes are kept as written; a vendor or
// product that needs one does not equal the plain names in the record.
func splitCPE(s string) (part, vendor, product, version string, err error) {
	var fields []string
	var cur strings.Builder
	escaped := false
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			cur.WriteRune(r)
			escaped = true
		case r == ':':
			fields = append(fields, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	fields = append(fields, cur.String())
	if len(fields) != 13 || fields[0] != "cpe" || fields[1] != "2.3" {
		return "", "", "", "", fmt.Errorf("not a CPE 2.3 formatted string")
	}
	return fields[2], fields[3], fields[4], fields[5], nil
}
