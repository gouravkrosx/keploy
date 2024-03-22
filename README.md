
<p align="center">
  <img align="center" src="https://docs.keploy.io/img/keploy-logo-dark.svg?s=200&v=4" height="40%" width="40%"  alt="keploy logo"/>
</p>
<h3 align="center">
<b>
⚡️ Backend tests faster than unit tests, from user traffic ⚡️
</b>
</h3 >
<p align="center">
🌟 The must-have tool for developers in the AI-Gen era 🌟
</p>

---

<h4 align="center">

<a href="https://keploy.io/"><img src="https://img.shields.io/website?url=https://keploy.io/&up_message=Keploy&up_color=%232635F1&label=Website&down_color=%232635F1&down_message=Keploy"></a>
<a href="https://Keploy.io/docs">
<img src="https://img.shields.io/badge/Join-Community!-orange" alt="Join our Community!" />
</a>
<a href="https://join.slack.com/t/keploy/shared_invite/zt-2dno1yetd-Ec3el~tTwHYIHgGI0jPe7A" alt="Slack">
<img src=".github/slack.svg" /></a>


   <a href="https://twitter.com/Keploy_io">
    <img src="https://img.shields.io/badge/follow-%40keployio-1DA1F2?logo=twitter&style=social" alt="Keploy Twitter" />
  </a>

<a href="https://github.com/Keploy/Keploy/issues">
    <img src="https://img.shields.io/github/stars/keploy/keploy?color=%23EAC54F&logo=github&label=Help us reach 4k stars! Now at:" alt="Help us reach 4k stars!" />
  </a>

  <a href="https://landscape.cncf.io/?item=app-definition-and-development--continuous-integration-delivery--keploy">
    <img src="https://img.shields.io/badge/CNCF%20Landscape-5699C6?logo=cncf&style=social" alt="Keploy CNCF Landscape" />
  </a>
</h4>

## 🎤 Introducing Keploy 🐰
[Keploy](https://keploy.io) is a **developer-centric** API testing tool that creates **backend tests along with built-in-mocks**, faster than unit tests.

Keploy record API calls and replays them during testing, making it **easy to use, powerful, and extensible**.  Here are Keploy's core features: 🛠

<img src="https://raw.githubusercontent.com/keploy/docs/main/static/gif/record-tc.gif" width="80%" alt="Convert API calls to test cases"/>

- ♻️ **Combined Test Coverage:** Merge your Keploy Tests with your fave testing libraries(JUnit, go-test, py-test, jest) to see a combined test coverage.


- 🤖 **EBPF Instrumentation:** Keploy uses EBPF like a secret sauce to make integration code-less, language-agnostic, and oh-so-lightweight.


- 🌐 **CI/CD Integration:** Run tests with mocks anywhere you like—locally on the CLI, in your CI pipeline (Jenkins, Github Actions..) , or even across a Kubernetes cluster.


- 📽️ **Record-Replay Complex Flows:** Keploy can record and replay complex, distributed API flows as mocks and stubs. It's like having a time machine for your tests—saving you tons of time!


- 🎭 **Multi-Purpose Mocks:** You can also use keploy Mocks, as server Tests!

> 🐰 **Fun fact:** Keploy uses itself for testing! Check out our swanky coverage badge: [![Coverage Status](https://coveralls.io/repos/github/keploy/keploy/badge.svg?branch=main&kill_cache=1)](https://coveralls.io/github/keploy/keploy?branch=main&kill_cache=1) &nbsp;


## 🎩 How's the Magic Happen?
Keploy proxy captures and replays **ALL**(CRUD operations, including non-idempotent APIs) of your app's network interactions.


Take a journey to **[How Keploy Works?](https://keploy.io/docs/keploy-explained/how-keploy-works/)** to discover the tricks behind the curtain!

<img src="https://raw.githubusercontent.com/keploy/docs/main/static/gif/record-replay.gif" width="100%" alt="Record Replay Testing"/>


## 📘 Documentation!
Become a Keploy pro with **[Keploy Documentation](https://keploy.io/docs/)**.


## 🛠️ Platform-Specific Requirements for Keploy

Below is a table summarizing the tools needed for both native and Docker installations of Keploy on MacOS, Windows, and Linux:

| Operating System                                                                                                                                                                                                                                                                                              | Without Docker                                                                                                                  | Docker Installation                                                                                                             | Prerequisites                                                                                                                                                                            |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| <img src="https://www.pngplay.com/wp-content/uploads/3/Apple-Logo-Transparent-Images.png" width="15" height="15" alt="MacOS" /> **MacOS**                                                                                                                                                                     | <img src="https://upload.wikimedia.org/wikipedia/en/b/ba/Red_x.svg" width="20" height="20" alt="Not Supported" />               | <img src="https://upload.wikimedia.org/wikipedia/commons/e/e5/Green_tick_pointed.svg" width="20" height="20" alt="Supported" /> | Docker Desktop version must be 4.25.2 or above                                                                                                                                           |
| <img src="https://upload.wikimedia.org/wikipedia/commons/5/5f/Windows_logo_-_2012.svg" width="15" height="15" alt="Windows" /> **Windows**                                                                                                                                                                    | <img src="https://upload.wikimedia.org/wikipedia/commons/e/e5/Green_tick_pointed.svg" width="20" height="20" alt="Supported" /> | <img src="https://upload.wikimedia.org/wikipedia/commons/e/e5/Green_tick_pointed.svg" width="20" height="20" alt="Supported" /> | - Use [WSL](https://learn.microsoft.com/en-us/windows/wsl/install#install-wsl-command) `wsl --install` <br/> - Windows 10 version 2004 and higher (Build 19041 and higher) or Windows 11 |
| <img src="https://th.bing.com/th/id/R.7802b52b7916c00014450891496fe04a?rik=r8GZM4o2Ch1tHQ&riu=http%3a%2f%2f1000logos.net%2fwp-content%2fuploads%2f2017%2f03%2fLINUX-LOGO.png&ehk=5m0lBvAd%2bzhvGg%2fu4i3%2f4EEHhF4N0PuzR%2fBmC1lFzfw%3d&risl=&pid=ImgRaw&r=0" width="10" height="10" alt="Linux" /> **Linux** | <img src="https://upload.wikimedia.org/wikipedia/commons/e/e5/Green_tick_pointed.svg" width="20" height="20" alt="Supported" /> | <img src="https://upload.wikimedia.org/wikipedia/commons/e/e5/Green_tick_pointed.svg" width="20" height="20" alt="Supported" /> | Linux kernel 5.15 or higher                                                                                                                                                              |

On MacOS and Windows, additional tools are required for Keploy due to the lack of native eBPF support.

# 🚀 Quick Installation

Integrate Keploy by installing the agent locally. No code-changes required.

```shell
curl -O https://raw.githubusercontent.com/keploy/keploy/main/keploy.sh && source keploy.sh
```

##  🎬 Recording Testcases

Start your app wit Keploy to convert API calls as Tests and Mocks/Stubs.

```zsh
keploy record -c "CMD_TO_RUN_APP" 
```
For example, if you're using a simple Python app the **CMD_TO_RUN_APP** would resemble to `python main.py`, for  Golang `go run main.go`, for java `java -jar xyz.jar`, for node `npm start`..

```zsh
keploy record -c "python main.py"
```

## 🧪 Running Tests
Shut down the databases, redis, kafka or any other services your application uses. Keploy doesn't need those during test.
```zsh
keploy test -c "CMD_TO_RUN_APP" --delay 10
```

## ✅ Test Coverage Integration
To integrate with your unit-testing library and see combine test coverage, follow this [test-coverage guide](https://keploy.io/docs/server/sdk-installation/go/).

> ####  **If You Had Fun:** Please leave a 🌟 star on this repo!  It's free, and you'll bring a smile. 😄 👏

## 🤔 Questions?
Reach out to us. We're here to help!

[![Slack](https://img.shields.io/badge/Slack-4A154B?style=for-the-badge&logo=slack&logoColor=white)](https://join.slack.com/t/keploy/shared_invite/zt-2dno1yetd-Ec3el~tTwHYIHgGI0jPe7A)
[![LinkedIn](https://img.shields.io/badge/linkedin-%230077B5.svg?style=for-the-badge&logo=linkedin&logoColor=white)](https://www.linkedin.com/company/keploy/)
[![YouTube](https://img.shields.io/badge/YouTube-%23FF0000.svg?style=for-the-badge&logo=YouTube&logoColor=white)](https://www.youtube.com/channel/UC6OTg7F4o0WkmNtSoob34lg)
[![Twitter](https://img.shields.io/badge/Twitter-%231DA1F2.svg?style=for-the-badge&logo=Twitter&logoColor=white)](https://twitter.com/Keployio)


## 🌐 Language Support
From Go's gopher 🐹 to Python's snake 🐍, we support:

![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white)
![Java](https://img.shields.io/badge/java-%23ED8B00.svg?style=for-the-badge&logo=java&logoColor=white)
![NodeJS](https://img.shields.io/badge/node.js-6DA55F?style=for-the-badge&logo=node.js&logoColor=white)
![Rust](https://img.shields.io/badge/Rust-darkred?style=for-the-badge&logo=rust&logoColor=white)
![C#](https://img.shields.io/badge/csharp-purple?style=for-the-badge&logo=csharp&logoColor=white)
![Python](https://img.shields.io/badge/python-3670A0?style=for-the-badge&logo=python&logoColor=ffdd54)


## 🫰 Let's Build Together! 🧡
Whether you're a newbie coder or a wizard 🧙‍♀️, your perspective is golden. Take a peek at our:

📜 [Contribution Guidelines](https://github.com/keploy/keploy/blob/main/CONTRIBUTING.md)

❤️ [Code of Conduct](https://github.com/keploy/keploy/blob/main/CODE_OF_CONDUCT.md)


## 🐲 Current Limitations!
- **Unit Testing:** While Keploy is designed to run alongside unit testing frameworks (Go test, JUnit..) and can add to the overall code coverage, it still generates integration tests.
- **Production Lands**: Keploy is currently focused on generating tests for developers. These tests can be captured from any environment, but we have not tested it on high volume production environments. This would need robust deduplication to avoid too many redundant tests being captured. We do have ideas on building a robust deduplication system [#27](https://github.com/keploy/keploy/issues/27)

## ✨ Resources!
🤔 [FAQs](https://keploy.io/docs/keploy-explained/faq/)

🕵️‍️ [Why Keploy](https://keploy.io/docs/keploy-explained/why-keploy/)

⚙️ [Installation Guide](https://keploy.io/docs/application-development/)

📖 [Contribution Guide](https://keploy.io/docs/keploy-explained/contribution-guide/)