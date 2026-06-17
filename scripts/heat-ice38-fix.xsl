<?xml version="1.0" encoding="utf-8"?>
<!--
  Heat output post-processor: makes harvested components ICE38-compliant.

  Per Microsoft docs (https://learn.microsoft.com/en-us/windows/win32/msi/ice38):
  "ICE38 validates that every component being installed under the current user's
   profile also specifies a registry key under the HKEY_CURRENT_USER root in the
   KeyPath column of the Component table."

  By default, heat.exe dir generates one Component per file with the File as the
  KeyPath. Since runtime/skills/ and runtime/agents/ install to LocalAppDataFolder
  (user profile), every harvested component triggers ICE38.

  This XSLT transform:
    1. Strips KeyPath="yes" from any File child.
    2. Strips the KeyPath attribute from the Component itself (if present).
    3. Appends a RegistryValue under HKCU as the new KeyPath.

  Wired into heat.exe via the -t <xsl> flag:
      heat.exe dir ".\runtime\skills" -t scripts/heat-ice38-fix.xsl ...

  Reference:
    - ICE38:  https://learn.microsoft.com/en-us/windows/win32/msi/ice38
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

  <!-- Component: copy attributes except KeyPath, copy children, append RegistryValue. -->
  <xsl:template match="wix:Component">
    <xsl:copy>
      <xsl:apply-templates select="@*[name() != 'KeyPath']"/>
      <xsl:apply-templates select="node()"/>
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
