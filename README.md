# PRTY 
***Make reviewing PRs a PARTY 🎉***

PRTY is your new favorite tool for more effective PR reviewing. It determines the best open PRs for you to review and in what order. It drastically reduces the time to review for PRs in states that need eyes.


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

You will need to create a github token that has `repo` and `read:org` permissions. If your organization requires SSO you **must** complete that flow for prty to be able to pull data. 

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


### How PRTY calculates **Importance**
The algorithm can be reviewed [here](https://github.com/ajones/prty/blob/main/datasource/pulls.go#L126). It normilizes all feature calculations to a range from 0-100 then sums them all up to determine the importance value for each PR. This is used for sort order in each tab, highest imporanct at the top.

**If you have ideas on how to improve this approach please put togeather a POC and make a PR!**


### Future Improvements ⚡️
If you are inspired, pick one and open a PR!
- Add CODEOWNERS detection in the importance calculation
- Automatically generate Personal Access Token
- Expand settings page to top author opens. Data already in stats file.
- Add `importance` per feature to the PR detail screen. Thinking a low % width column on the right side...🤷
- Improve refresh algorithm to reduce number of new calls needed on refresh
- Refactor out repeated code of the PR UI's
- Loading animations... or heck any animations anywhere!
- Should the visible tabs be configurable? 
