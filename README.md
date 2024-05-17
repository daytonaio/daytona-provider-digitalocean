<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://github.com/daytonaio/daytona/raw/main/assets/images/Daytona-logotype-white.png">
    <img alt="Daytona logo" src="https://github.com/daytonaio/daytona/raw/main/assets/images/Daytona-logotype-black.png" width="40%">
  </picture>
</div>

<br/>

<div align="center">

[![License](https://img.shields.io/badge/License-MIT-blue)](#license)
[![Go Report Card](https://goreportcard.com/badge/github.com/daytonaio/daytona)](https://goreportcard.com/report/github.com/daytonaio/daytona)
[![Issues - daytona](https://img.shields.io/github/issues/daytonaio/daytona)](https://github.com/daytonaio/daytona/issues)
![GitHub Release](https://img.shields.io/github/v/release/daytonaio/daytona)

</div>


<h1 align="center">Daytona DigitalOcean Provider</h1>
<div align="center">
This repository is the home of the <a href="https://github.com/daytonaio/daytona">Daytona</a> DigitalOcean Provider.
</div>
</br>

<p align="center">
  <a href="https://github.com/github.com/daytonaio/daytona-provider-digitalocean/issues/new?assignees=&labels=bug&projects=&template=bug_report.md&title=%F0%9F%90%9B+Bug+Report%3A+">Report Bug</a>
    ·
  <a href="https://github.com/github.com/daytonaio/daytona-provider-digitalocean/issues/new?assignees=&labels=enhancement&projects=&template=feature_request.md&title=%F0%9F%9A%80+Feature%3A+">Request Feature</a>
    ·
  <a href="https://join.slack.com/t/daytonacommunity/shared_invite/zt-273yohksh-Q5YSB5V7tnQzX2RoTARr7Q">Join Our Slack</a>
    ·
  <a href="https://twitter.com/Daytonaio">Twitter</a>
</p>

The DigitalOcean Provider allows Daytona to create workspace projects on Digital Ocean VMs known as droplets.

## Target Options

| Property   	| Type   	| Optional 	| DefaultValue     	| InputMasked 	| DisabledPredicate 	|
|------------	|--------	|----------	|------------------	|-------------	|-------------------	|
| Auth Token 	| String 	| true     	|                  	| true        	|                   	|
| Disk Size  	| Int    	| false    	| 20               	| false       	|                   	|
| Image      	| String 	| false    	| ubuntu-22-04-x64 	| false       	|                   	|
| Region     	| String 	| false    	| fra1             	| false       	|                   	|
| Size       	| String 	| false    	| s-2vcpu-4gb      	| false       	|                   	|

### Default Targets

The Digital Ocean Provider has no default targets.

## Code of Conduct

This project has adapted the Code of Conduct from the [Contributor Covenant](https://www.contributor-covenant.org/). For more information see the [Code of Conduct](CODE_OF_CONDUCT.md) or contact [codeofconduct@daytona.io.](mailto:codeofconduct@daytona.io) with any additional questions or comments.

## Contributing

The Daytona DigitalOcean Provider is Open Source under the [MIT License](LICENSE). If you would like to contribute to the software, you must:

1. Read the Developer Certificate of Origin Version 1.1 (https://developercertificate.org/)
2. Sign all commits to the Daytona DigitalOcean Provider project.

This ensures that users, distributors, and other contributors can rely on all the software related to Daytona being contributed under the terms of the [License](LICENSE). No contributions will be accepted without following this process.

Afterwards, navigate to the [contributing guide](CONTRIBUTING.md) to get started.

## Questions

For more information on how to use and develop Daytona, talk to us on
[Slack](https://join.slack.com/t/daytonacommunity/shared_invite/zt-273yohksh-Q5YSB5V7tnQzX2RoTARr7Q).
