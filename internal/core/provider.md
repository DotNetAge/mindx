# 关于Provider的API_Key的说明

## 规则

1. Provider 的 APIKey 与 AuthToken 保存的永远只有环境变量，只是用于程序初始化时使用，真正的API_KEY是不会以明码方式保存至到`settings/providers.yml`中。
2. TUI 在设置AuthKey之前会先一次性根据Provider中的APIKey的定义一次性从环境变量中读取具体值，如果有值就会直接将这个值保存到 `CredentialStore`中，以 Provider.Name 为键。
3. TUI不应该对Provider的APIKey值进行任何设置；
4. WebUI 在收到来自客户端的设置Key的请求时，也先会从环境变量中以用户提供的APIKey作为键，读取具体值，如果有值就会直接将这个值保存到 `CredentialStore`中，如果没有就以用户提供的值作为KEY值直接保存至 `CredentialStore`中。
5. 当 App 向 GoReact 的runtime进行构造时应该以 model.provider为键，从 `CredentialStore`中读取 APIKey。
