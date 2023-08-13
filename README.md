# Grab

[![GitHub go.mod Go version of a Go module](https://img.shields.io/github/go-mod/go-version/willnilges/grab.svg)](https://github.com/willnilges/grab)
![You actually did ask for this](https://img.shields.io/badge/you_actually-asked_for_this-red)
![Made with Fury](https://img.shields.io/badge/made_with-fury-orange)

<p align="center">
  <img height="300px" src="https://github.com/WillNilges/grab/blob/8aa627c6dc2e49be53e4b897033c35795fc28399/static/images/grabbit_head.png" alt="Oxford the Grabbit">
</p>

> be me
> 
> have massive issue with xyz service
> 
> have 600+ message thread in Slack to discuss problem
> 
> fix problem, generate tons of useful information
> 
> "I'm sure we'll never have this problem again."
> 
> mfw problem happens again
> 
> mfw waste time digging through slack for that thread again

### Stop loosing sh*t!

https://github.com/WillNilges/grab/blob/8aa627c6dc2e49be53e4b897033c35795fc28399/static/images/grabbit_head.png

Grab is a cross-platform* application that connects your messaging platformm to your knowledge base. Simply ask the bot, and easily transfer knowledge generated in your messaging platform to a proper, more permanent home. Slack might be expensive, but wisdom can be priceless!

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

I've had the above situation play out so many times throughout my journey as a developer, and I'm tired of it. About a year ago, I had the idea for this project, and then a friend told me that there are already [plenty](https://www.getguru.com/) [of](https://www.backupery.com/products/backupery-for-slack/) [startups](http://landria.io/) charging people $N/seat/month or some other ludicrous amount for this kind of service. I think that's way too much, so I made my own.

There are many communities and businesses out there, working on incredible things, but have poor documentation practices. Information gets lost, links get buried, and your employees will waste time searching for them again when they may or may not exist. This app aims to drastically lower the barrier to entry for good documentation, and and help people spend less time searching and more time doing.


### Status

Grab is currently under development and absolutely not ready for public consumption. Check the [issues](https://github.com/WillNilges/grab/issues) tab for progress on development. **I am literally just one guy and would very much like some help.**

### Roadmap™
- AI summarization
- Confluence integration
- Discord integration
- BookStack integration
- MS Teams integration
- SharePoint integration
- Uhhh IDK profit?

### Setup

This app is designed to be containerized and deployed on OpenShift or other K8s-flavored platform. Deploy it from Git, build it, and provide the environment variables listed in `.env.template.`

In the `.env` file, You MUST use `<wiki url>/api.php` to point to your wiki!!!

### Credits

[Christine Espeleta](https://github.com/chrissye0) for creating Oxford, the Grabbit!
