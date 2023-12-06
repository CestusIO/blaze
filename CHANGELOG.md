# CHANGELOG

This CHANGELOG is a format conforming to [keep-a-changelog](https://github.com/olivierlacan/keep-a-changelog). 
It is generated with git-chglog -o CHANGELOG.md
It assumes the use of [conventional commits](https://www.conventionalcommits.org/)

<a name="unreleased"></a>
## [Unreleased]


<a name="v0.7.2"></a>
## [v0.7.2]
### Chores
- add version deduction from version.yaml


<a name="v0.7.1"></a>
## [v0.7.1]
### Bug Fixes
- forgot to increase version number


<a name="v0.7.0"></a>
## [v0.7.0]
### Chores
- release version 0.7.0

### Features
- update otel versions


<a name="v0.6.3"></a>
## [v0.6.3]
### Bug Fixes
- more fixes to goreleaser config


<a name="v0.6.2"></a>
## [v0.6.2]
### CI
- adapt .gitignore * it missed ignoring the checked out actions


<a name="v0.6.1"></a>
## [v0.6.1]
### Chores
- move to github


<a name="v0.6.0"></a>
## [v0.6.0]
### CI
- switch to libv2

### Features
- add servicegroup

### Pull Requests
- Merge branch 'servicegroup' into 'master'


<a name="v0.5.0"></a>
## [v0.5.0]
### Features
- Add Sample implementation of the server interface

### Pull Requests
- Merge branch 'sampleImp' into 'master'


<a name="v0.4.1"></a>
## [v0.4.1]
### Bug Fixes
- add InjectTracer call for all service methods to add the current tracer to the context so it can be reused by downstream methods

### Pull Requests
- Merge branch 'imp' into 'master'


<a name="v0.4.0"></a>
## [v0.4.0]
### Pull Requests
- Merge branch 'imp' into 'master'


<a name="v0.3.2"></a>
## [v0.3.2]
### Chores
- Remove use of deprecated ioutils package use io instead

### Pull Requests
- Merge branch 'deprecated' into 'master'


<a name="v0.3.1"></a>
## [v0.3.1]
### Bug Fixes
- Forgot to update version

### Pull Requests
- Merge branch 'vs' into 'master'


<a name="v0.3.0"></a>
## [v0.3.0]
### Features
- Update otel to 0.19.0

### Pull Requests
- Merge branch 'update' into 'master'


<a name="v0.2.3"></a>
## [v0.2.3]
### Pull Requests
- Merge branch 'config' into 'master'


<a name="v0.2.2"></a>
## [v0.2.2]
### Fix
- Do not JSON serialize the proto message

### Pull Requests
- Merge branch 'hotfic' into 'master'


<a name="v0.2.1"></a>
## [v0.2.1]
### Pull Requests
- Merge branch 'cleanup' into 'master'


<a name="v0.2.0"></a>
## [v0.2.0]
### Pull Requests
- Merge branch 'protov2' into 'master'


<a name="v0.1.1"></a>
## [v0.1.1]
### Pull Requests
- Merge branch 'cleanup' into 'master'


<a name="v0.1.0"></a>
## [v0.1.0]
### Pull Requests
- Merge branch 'gogo' into 'master'


<a name="v0.0.9"></a>
## [v0.0.9]
### Pull Requests
- Merge branch 'fix' into 'master'


<a name="v0.0.8"></a>
## [v0.0.8]
### Pull Requests
- Merge branch 'trace' into 'master'
- Merge branch 'trace' into 'master'


<a name="v0.0.7"></a>
## [v0.0.7]
### Pull Requests
- Merge branch 'trace' into 'master'


<a name="v0.0.6"></a>
## [v0.0.6]
### Pull Requests
- Merge branch 'feature/trace' into 'master'


<a name="v0.0.5"></a>
## [v0.0.5]
### Pull Requests
- Merge branch 'feature/trace' into 'master'


<a name="v0.0.4"></a>
## [v0.0.4]
### Pull Requests
- Merge branch 'feature/client' into 'master'


<a name="v0.0.3"></a>
## [v0.0.3]
### Pull Requests
- Merge branch 'feature/client' into 'master'


<a name="v0.0.2"></a>
## [v0.0.2]
### Pull Requests
- Merge branch 'feature/client' into 'master'


<a name="v0.0.1"></a>
## v0.0.1
### Pull Requests
- Merge branch 'feat/generation' into 'master'
- Merge branch 'feature/helpers' into 'master'
- Merge branch 'feature/tests' into 'master'
- Merge branch 'feature/error' into 'master'
- Merge branch 'init' into 'master'


[Unreleased]: https://github.com/CestusIO/blaze.git/compare/v0.7.2...HEAD
[v0.7.2]: https://github.com/CestusIO/blaze.git/compare/v0.7.1...v0.7.2
[v0.7.1]: https://github.com/CestusIO/blaze.git/compare/v0.7.0...v0.7.1
[v0.7.0]: https://github.com/CestusIO/blaze.git/compare/v0.6.3...v0.7.0
[v0.6.3]: https://github.com/CestusIO/blaze.git/compare/v0.6.2...v0.6.3
[v0.6.2]: https://github.com/CestusIO/blaze.git/compare/v0.6.1...v0.6.2
[v0.6.1]: https://github.com/CestusIO/blaze.git/compare/v0.6.0...v0.6.1
[v0.6.0]: https://github.com/CestusIO/blaze.git/compare/v0.5.0...v0.6.0
[v0.5.0]: https://github.com/CestusIO/blaze.git/compare/v0.4.1...v0.5.0
[v0.4.1]: https://github.com/CestusIO/blaze.git/compare/v0.4.0...v0.4.1
[v0.4.0]: https://github.com/CestusIO/blaze.git/compare/v0.3.2...v0.4.0
[v0.3.2]: https://github.com/CestusIO/blaze.git/compare/v0.3.1...v0.3.2
[v0.3.1]: https://github.com/CestusIO/blaze.git/compare/v0.3.0...v0.3.1
[v0.3.0]: https://github.com/CestusIO/blaze.git/compare/v0.2.3...v0.3.0
[v0.2.3]: https://github.com/CestusIO/blaze.git/compare/v0.2.2...v0.2.3
[v0.2.2]: https://github.com/CestusIO/blaze.git/compare/v0.2.1...v0.2.2
[v0.2.1]: https://github.com/CestusIO/blaze.git/compare/v0.2.0...v0.2.1
[v0.2.0]: https://github.com/CestusIO/blaze.git/compare/v0.1.1...v0.2.0
[v0.1.1]: https://github.com/CestusIO/blaze.git/compare/v0.1.0...v0.1.1
[v0.1.0]: https://github.com/CestusIO/blaze.git/compare/v0.0.9...v0.1.0
[v0.0.9]: https://github.com/CestusIO/blaze.git/compare/v0.0.8...v0.0.9
[v0.0.8]: https://github.com/CestusIO/blaze.git/compare/v0.0.7...v0.0.8
[v0.0.7]: https://github.com/CestusIO/blaze.git/compare/v0.0.6...v0.0.7
[v0.0.6]: https://github.com/CestusIO/blaze.git/compare/v0.0.5...v0.0.6
[v0.0.5]: https://github.com/CestusIO/blaze.git/compare/v0.0.4...v0.0.5
[v0.0.4]: https://github.com/CestusIO/blaze.git/compare/v0.0.3...v0.0.4
[v0.0.3]: https://github.com/CestusIO/blaze.git/compare/v0.0.2...v0.0.3
[v0.0.2]: https://github.com/CestusIO/blaze.git/compare/v0.0.1...v0.0.2
