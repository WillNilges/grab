# Grab

[![GitHub go.mod Go version of a Go module](https://img.shields.io/github/go-mod/go-version/willnilges/grab.svg)](https://github.com/willnilges/grab)
![You actually did ask for this](https://img.shields.io/badge/you_actually-asked_for_this-red)
![Made with Extreme Frustration](https://img.shields.io/badge/made_with-extreme_frustration-orange)
![Contains Rabbits](https://img.shields.io/badge/contains-rabbits-green)
![Hosted By CSH](https://img.shields.io/badge/hosted_by-csh.rit.edu-pink?logo=data:image/svg%2bxml;base64,PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0iVVRGLTgiPz4KPCEtLSBHZW5lcmF0b3I6IEFkb2JlIElsbHVzdHJhdG9yIDEzLjAuMiwgU1ZHIEV4cG9ydCBQbHVnLUluICAtLT4KPCFET0NUWVBFIHN2ZyBQVUJMSUMgIi0vL1czQy8vRFREIFNWRyAxLjEvL0VOIiAiaHR0cDovL3d3dy53My5vcmcvR3JhcGhpY3MvU1ZHLzEuMS9EVEQvc3ZnMTEuZHRkIj4KPHN2ZyB2ZXJzaW9uPSIxLjEiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIgeG1sbnM6eGxpbms9Imh0dHA6Ly93d3cudzMub3JnLzE5OTkveGxpbmsiIHhtbG5zOmE9Imh0dHA6Ly9ucy5hZG9iZS5jb20vQWRvYmVTVkdWaWV3ZXJFeHRlbnNpb25zLzMuMC8iIHg9IjBweCIgeT0iMHB4IiB3aWR0aD0iNTQwcHgiIGhlaWdodD0iNTI1cHgiIHZpZXdCb3g9Ii0wLjAxNCAwLjAyOCA1NDAgNTI1IiBlbmFibGUtYmFja2dyb3VuZD0ibmV3IC0wLjAxNCAwLjAyOCA1NDAgNTI1IiB4bWw6c3BhY2U9InByZXNlcnZlIj4KPGRlZnM+CjwvZGVmcz4KPHBhdGggZmlsbC1ydWxlPSJldmVub2RkIiBjbGlwLXJ1bGU9ImV2ZW5vZGQiIGQ9Ik00MjAsMTM1VjMwYzAtMTYuNTY5LTEzLjQzMy0zMC0zMC0zMEgzMEMxMy40MzEsMCwwLDEzLjQzMSwwLDMwVjQ5NSAgYzAsMTYuNTY3LDEzLjQzMSwzMCwzMCwzMGgzNjBjMTYuNTY3LDAsMzAtMTMuNDMzLDMwLTMwVjM5MGgtOTBWNDIwYzAsOC4yODQtNi43MTUsMTUtMTUsMTVIMTA1Yy04LjI4NCwwLTE1LTYuNzE2LTE1LTE1VjEwNSAgYzAtOC4yODQsNi43MTYtMTUsMTUtMTVoMjEwYzguMjg1LDAsMTUsNi43MTYsMTUsMTV2MzBINDIweiIvPgo8cGF0aCBkPSJNMjkyLjUsMTE5Ljk5OUgxMjcuNTAxYy00LjE0MywwLTcuNSwzLjM1Ny03LjUsNy41VjI4NWMwLDQuMTQzLDMuMzU3LDcuNDk5LDcuNSw3LjQ5OWgxMDEuMjVjMi4wNzEsMCwzLjc1LDEuNjgsMy43NSwzLjc1MSAgdjQ0Ljk5OWMwLDIuMDcxLTEuNjc5LDMuNzUtMy43NSwzLjc1SDE5MS4yNWMtMi4wNzEsMC0zLjc1LTEuNjc5LTMuNzUtMy43NVYzMjIuNWgtNjcuNDk5VjM5Ny41YzAsNC4xNDIsMy4zNTcsNy41LDcuNSw3LjVIMjkyLjUgIGM0LjE0MywwLDcuNS0zLjM1OCw3LjUtNy41VjI0MGMwLTQuMTQzLTMuMzU3LTcuNS03LjUtNy41SDE5MS4yNWMtMi4wNzEsMC0zLjc1LTEuNjc5LTMuNzUtMy43NWwtMC4wMDEtNDUgIGMwLTIuMDcxLDEuNjc5LTMuNzUsMy43NS0zLjc1aDM3LjUwMWMyLjA3MSwwLDMuNzUsMS42NzksMy43NSwzLjc1bDAuMDAxLDE4Ljc1SDMwMHYtNzUuMDAxICBDMzAwLDEyMy4zNTYsMjk2LjY0MywxMTkuOTk5LDI5Mi41LDExOS45OTl6Ii8+Cjxwb2x5Z29uIHBvaW50cz0iNDIwLDMwMCA0MjAsMzYwIDMzMCwzNjAgMzMwLDE2NSA0MjAsMTY1IDQyMCwyMjUgNDUwLDIyNSA0NTAsMCA1NDAsMCA1NDAsNTI1IDQ1MCw1MjUgNDUwLDMwMCAiLz4KPC9zdmc+)

<p align="center">
  <img height="300px" src="https://github.com/WillNilges/grab/blob/8aa627c6dc2e49be53e4b897033c35795fc28399/static/images/grabbit_head.png" alt="Oxford the Grabbit">
</p>

**Dear `$text_based_chat_platform` Users,**

Does this sound familiar?

> Be me, at work, probably on a Friday
>
> Have a massive problem with xyz service. Maybe it's an outage, maybe you're scrambling to update dependencies; Either way, it's bad
> 
> Have 600+ message thread in Slack to discuss the problem
> 
> Fix the problem, generate tons of useful information
> 
> "I'm sure we'll never have this problem again."
> 
> mfw problem happens again
> 
> mfw waste 30+ minutes digging through slack for that thread that may-or-may-not exist again

**It doesn't have to be this way.**

### Grab helps you preserve information.

https://github.com/WillNilges/grab/blob/8aa627c6dc2e49be53e4b897033c35795fc28399/static/images/grabbit_head.png

Grab is a cross-platform* application that connects your messaging platform to your knowledge base. Simply ask the bot, and easily transfer knowledge generated in your messaging platform to a proper, more permanent home. Slack might be expensive, but information can be priceless!

*Not cross-platform yet lol

<table>
  <tr>
    <td colspan="2">Messaging</td>
    <td colspan="2">Knowledge Base</td>
  </tr>
  <tr>
    <td>Slack</td>
    <td> ✅ </td>
    <td>MediaWiki</td>
    <td> ✅ </td>
  </tr>
  <tr>
    <td>Discord</td>
    <td> ❌ </td>
    <td>Confluence</td>
    <td> ❌ </td>
  </tr>
    <tr>
    <td>MS Teams</td>
    <td> ❌ </td>
    <td>SharePoint</td>
    <td> ❌ </td>
  </tr>
    </tr>
    <tr>
    <td>Matrix</td>
    <td> ❌ </td>
    <td><a href="https://www.dokuwiki.org/dokuwiki">DokuWiki</a></td>
    <td> ❌ </td>
  </tr>
  </tr>
    <td>Zulip </td>
    <td> ❌ </td>
    <td><a href="https://www.bookstackapp.com/">BookStack</a></td>
    <td> ❌ </td>
  </tr>
</table>

### Why?

I've had the situation at the top of this page play out so many times throughout my journey as a developer, and honestly, I'm tired of it. About a year ago, I had the idea for this project, and then a friend told me that there are already [plenty](https://www.getguru.com/) [of](https://www.backupery.com/products/backupery-for-slack/) [startups](http://landria.io/) charging people $N/seat/month or some other ludicrous amount for this kind of service. I think that's way too much, so I made my own. Graciously hosted by [Computer Science House](https://csh.rit.edu)

There are many communities and businesses out there, working on incredible things, but have poor documentation practices. Information gets lost, links get buried, and your employees will waste time searching for them again when they may or may not even exist. This app aims to drastically lower the barrier to entry for good documentation, and and help people spend less time searching and more time doing.


### Status

Grab is currently under development and absolutely not ready for public consumption. Check the [issues](https://github.com/WillNilges/grab/issues) tab for progress on development. **I am literally just one guy and would very much like some help.**

### Roadmap™
- AI summarization
- Confluence integration
- Discord integration
- BookStack integration
- MS Teams integration
- SharePoint integration
- Charge a menial fee for server hosting if the app gets too big.

_Full transparency: This app is free software, GPL3'ed. I want it to stay that way forever. Currently, it's being graciously hosted by [Computer Science House](https://csh.rit.edu) at RIT, but if it gets too big, I might have to move it to AWS, which will be ✨expensive✨. I never intend to make money off of this, but in that event, I'll start asking for donations, or perhaps charge a break-even price for hosting. Hopefully no more than $10/month, flat, regardless of org size._

### Setup

This app is designed to be containerized and deployed on OpenShift or other K8s-flavored platform. Deploy it from Git, build it, and provide the environment variables listed in `.env.template.`

In the `.env` file, You MUST use `<wiki url>/api.php` to point to your wiki!!!

### Credits

[Christine Espeleta](https://github.com/chrissye0) for creating Oxford, the Grabbit!
