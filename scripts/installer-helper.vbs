' MindX Installer VBScript Helper
' Called by WiX CustomAction to parse SELECTED_PROVIDER value
'
' Input:  Session.Property("SELECTED_PROVIDER") = "openai|OPENAI_API_KEY"
' Output: Session.Property("ENV_VAR_NAME") = "OPENAI_API_KEY"

Function SplitProviderValue()
  On Error Resume Next

  Dim rawValue
  rawValue = Session.Property("SELECTED_PROVIDER")

  If IsNull(rawValue) Or rawValue = "" Then
    ' Default fallback — shouldn't happen since default is set in wxs
    Session.Property("ENV_VAR_NAME") = "OPENAI_API_KEY"
    SplitProviderValue = 0  ' MSI success (0 = success for VBScript CA)
    Exit Function
  End If

  ' Split on "|" to extract the environment variable name (part after pipe)
  Dim parts
  parts = Split(rawValue, "|")

  If UBound(parts) >= 1 Then
    Session.Property("ENV_VAR_NAME") = Trim(parts(1))
  Else
    ' No pipe found — use raw value as-is (backward compat)
    Session.Property("ENV_VAR_NAME") = Trim(rawValue)
  End If

  SplitProviderValue = 0  ' MSI success (0 = success for VBScript CA)
End Function
