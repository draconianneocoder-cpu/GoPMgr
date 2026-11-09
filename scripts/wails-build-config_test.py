#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
import importlib.util
import pathlib
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("build_config", pathlib.Path(__file__).with_name("wails-build-config.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class ProductMetadataTests(unittest.TestCase):
    def test_canonical_config_and_invalid_values(self):
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "config.yml"
            path.write_text('# product metadata fixture\n{"version":"3","info":{"version":"1.1.0"}}')
            self.assertEqual(module.read_info(path, "version"), "1.1.0")
            for content in (
                '{"version":"2","info":{"version":"1.1.0"}}',
                '{"version":"3","info":{"version":3}}',
                '{"version":"3","info":{"version":""}}',
                '{"version":"3","info":{"version":"1.1.0\\nshadow"}}',
            ):
                path.write_text(content)
                with self.subTest(content=content), self.assertRaises(ValueError):
                    module.read_info(path, "version")


if __name__ == "__main__":
    unittest.main()
