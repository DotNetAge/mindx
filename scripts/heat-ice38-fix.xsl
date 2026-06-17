<?xml version="1.0" encoding="utf-8"?>
<!--
  Heat output post-processor: makes harvested components ICE38/ICE64-compliant.

  ICE38 (per https://learn.microsoft.com/en-us/windows/win32/msi/ice38):
  "Every component installed under the current user's profile must specify a
   registry key under HKEY_CURRENT_USER as its KeyPath, not a file."

  ICE64 (per https://learn.microsoft.com/en-us/windows/win32/msi/ice64):
  "New directories in the user profile must be removed correctly in roaming
   scenarios." Each such directory must have a row in the RemoveFile table.

  heat.exe dir by default:
    * Creates one Component per file with the File as KeyPath (violates ICE38)
    * Creates subdirectories under user profile without RemoveFile (violates ICE64)

  This XSLT transform:
    1. Strips KeyPath="yes" from any File child.
    2. Strips the KeyPath attribute from the Component itself (if present).
    3. Appends a RegistryValue under HKCU as the new KeyPath.
    4. Appends a RemoveFile to clean up the Component's directory on uninstall.

  Wired into heat.exe via the -t <xsl> flag:
      heat.exe dir ".\runtime\skills" -t scripts/heat-ice38-fix.xsl ...

  References:
    - ICE38: https://learn.microsoft.com/en-us/windows/win32/msi/ice38
    - ICE64: https://learn.microsoft.com/en-us/windows/win32/msi/ice64
    - heat -t: https://docs.firegiant.com/wix3/overview/heat/
-->
<xsl:stylesheet
  version="1.0"
  xmlns="http://schemas.microsoft.com/wix/2006/wi"
  xmlns:xsl="http://www.w3.org/1999/XSL/Transform"
  xmlns:wix="http://schemas.microsoft.com/wix/2006/wi"
  exclude-result-prefixes="wix">

  <xsl:output encoding="utf-8" method="xml" version="1.0" indent="yes"/>

  <!-- Identity: copy everything as-is. -->
  <xsl:template match="@*|node()">
    <xsl:copy>
      <xsl:apply-templates select="@*|node()"/>
    </xsl:copy>
  </xsl:template>

  <!-- Drop KeyPath="yes" from File elements (force File to NOT be the KeyPath). -->
  <xsl:template match="wix:File/@KeyPath"/>

  <!-- Component: copy attributes except KeyPath, copy children, append RemoveFile + RegistryValue. -->
  <xsl:template match="wix:Component">
    <xsl:copy>
      <xsl:apply-templates select="@*[name() != 'KeyPath']"/>
      <xsl:apply-templates select="node()"/>
      <!-- Directory attribute is intentionally omitted: in heat -srd output the
           Component lives inside a <Directory> element and has no @Directory
           attribute of its own. The RemoveFile default (per
           https://docs.firegiant.com/wix3/xsd/wix/removefile/) is the parent
           component's directory, which is exactly the <Directory> wrapping
           this Component. This creates a RemoveFile row per harvested
           subdirectory and resolves ICE64. -->
      <RemoveFile Id="Rm_{@Id}" Name="*" On="uninstall"/>
      <RegistryValue
        Id="Reg_{@Id}"
        Root="HKCU"
        Key="Software\DotNetAge\mindx\InstallState\HeatComponents"
        Name="{@Id}"
        Type="integer"
        Value="1"
        KeyPath="yes"/>
    </xsl:copy>
  </xsl:template>
</xsl:stylesheet>
