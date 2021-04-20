# PRTY 
Make reviewing PRs a PARTY 🎉


### Installation
#### Mac 🍎
PRTY is available via homebrew
```
$ brew tap ajones/prty
$ brew install prty
$ prty
```

#### From source 🛖
```
$ git clone git@github.com:ajones/prty.git
$ cd prty
$ go install ./...
$ ~/go/bin/prty
```

### Configuration 🛠
PRTY creates a folder in your home directory where it stores configuration and data cache. After the first run of the application it will point you a file where you need to add some info. 

You will need to create a github token that has repo read permission on the organizations and repositories that you wish to view.

```
cat ~/.prty/conf.yaml

...
GithubAccessToken: Place your Github tok here
GithubUsername: Place your Github username here
...
```

#### Configuration key information

| Key | Description |
| --- | ----------- |
| ConfigVersion | Version of this config file. Used when migrating forward  |
| GithubAccessToken | Token used when calling Github for data |
| GithubUsername | Your github username. Used to match your PRs in importance calculations |
| OrgWhitelist | (optional) Organizations to include **Leave Empty For All** |
| OrgBlacklist | (optional) Organizations to exclude |
| RepoWhitelist | (optional) Repos to include **Leave Empty For All** |
| OrgBlacklist | (optional) Repos to exclude |
| BotUsernames | (optional) If your org utilizes build or rennovate bots put their usernames here to have them appear on their own tab. Leave empty if you wish to have the Bot PRs appear weighted throught other tabs |
| TeamUsernames | (optional) Comma delimited list of team Github Usernames |
| AbandonedAgeDays | Number of days after last activity before PR is considered abandoned |


### Future Improvements ⚡️
If you are inspired, pick one and open a PR!
- ...