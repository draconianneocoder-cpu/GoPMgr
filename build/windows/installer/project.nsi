# SPDX-FileCopyrightText: 2018-Present Lea Anthony and the Wails contributors
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: MIT
#
# GoPMgr-owned Wails v2.13 NSIS entrypoint. Wails regenerates
# wails_tools.nsh from its pinned embedded template on every build; keeping
# project.nsi here prevents fallback to Wails' generic installer entrypoint.

Unicode true
SetCompressor /SOLID lzma

!include "wails_tools.nsh"

# Windows version resources require four numeric components. GoPMgr's version
# of record is clean three-part SemVer, so the build component remains zero.
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion "${INFO_PRODUCTVERSION}.0"
VIAddVersionKey "CompanyName" "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion" "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion" "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright" "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName" "${INFO_PRODUCTNAME}"

ManifestDPIAware true
BrandingText "GoPMgr"

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_ABORTWARNING
!define MUI_FINISHPAGE_NOAUTOCLOSE
!define MUI_FINISHPAGE_RUN "$INSTDIR\${PRODUCT_EXECUTABLE}"
!define MUI_FINISHPAGE_RUN_TEXT "Launch ${INFO_PRODUCTNAME}"
!define MUI_WELCOMEPAGE_TITLE "Install ${INFO_PRODUCTNAME}"
!define MUI_WELCOMEPAGE_TEXT "This wizard installs ${INFO_PRODUCTNAME}, a local-first project controls application.$\r$\n$\r$\nProject data remains on this computer and is not removed by uninstalling the application."

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "..\..\..\LICENSES\GPL-3.0-or-later.txt"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

# Signing hooks stay disabled until release certificates and timestamping
# credentials are configured in GitHub Actions.
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe"

!ifdef WAILS_INSTALL_SCOPE
  !if "${WAILS_INSTALL_SCOPE}" == "user"
    InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
  !else
    InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
  !endif
!else
  InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
!endif

ShowInstDetails show
ShowUninstDetails show

Function .onInit
  !insertmacro wails.checkArchitecture
FunctionEnd

Section "GoPMgr" SEC_GOPMGR
  !insertmacro wails.setShellContext
  !insertmacro wails.webview2runtime

  SetOutPath "$INSTDIR"
  !insertmacro wails.files

  CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
  CreateShortcut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

  !insertmacro wails.associateFiles
  !insertmacro wails.associateCustomProtocols
  !insertmacro wails.writeUninstaller
SectionEnd

Section "Uninstall"
  !insertmacro wails.setShellContext

  # Remove only the Wails WebView2 cache and installed program files. GoPMgr
  # project databases live in the user's Documents\GoPMgr tree (or the older
  # Documents\PMForge, for an install not yet migrated) and must survive
  # uninstall so an application upgrade cannot destroy user work.
  RMDir /r "$AppData\${PRODUCT_EXECUTABLE}"
  RMDir /r "$INSTDIR"

  Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
  Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

  !insertmacro wails.unassociateFiles
  !insertmacro wails.unassociateCustomProtocols
  !insertmacro wails.deleteUninstaller
SectionEnd
