# Grab

[![GitHub go.mod Go version of a Go module](https://img.shields.io/github/go-mod/go-version/willnilges/grab.svg)](https://github.com/willnilges/grab)
![You actually did ask for this](https://img.shields.io/badge/you_actually-asked_for_this-red)
![You actually did ask for this](https://img.shields.io/badge/made_with-anger-orange)

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
> mfw spend 30 minutes digging through slack for that thread again.

or, even better:

> mfw my corporate policy limits Slack retention to 30 days and it's been 6 months

### Stop loosing sh*t!

Grab is a cross-platform* application that connects your messaging app to your knowledge base. Simply ping the bot, and it will publish a transcript of a conversation to your wiki for you! This allows you to easily transfer knowledge to a proper, more permanent home.

*Not cross-platform yet lol

<table>
  <tr>
    <td colspan="2">Messaging</td>
    <td colspan="2">Knowledge Base</td>
  </tr>
  <tr>
    <td>Slack</td>
    <td>✅</td>
    <td>MediaWiki</td>
    <td>✅</td>
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
    <td>DokuWiki</td>
    <td> ❌ </td>
  </tr>
</table>

### Why?

I've had the above greentext play out so many times throughout my journey as a developer. I'm tired of it. About a year ago, I had this idea, and then I learned that there are [plenty](https://www.getguru.com/) [of](https://www.backupery.com/products/backupery-for-slack/) [startups](http://landria.io/) that are charging people $8-10/seat/month or some other ludicrous amount for this kind of service. I think that's way too much, so I made my own.

There are too many damn communities and businesses out there that have horrible documentation practices. I've seen many firsthand. They're a mess, becuase they keep forgetting things and having to re-learn them because they get buried in Discord or Slack or somewhere knowledge is not supposed to remain for a long time. This app aims to drastically lower the barrier to entry for good documentation, and help more people STOP LOOSING SH*T!!!

### Status

Grab is currently under development and absolutely not ready for public consumption. Check the [issues](https://github.com/WillNilges/grab/issues) tab for progress on development. **I am literally just one guy and would very much like some help.**

### Roadmap™
- AI summarization
- Confluence integration
- Discord integration
- MS Teams integration
- SharePoint integration
- Uhhh IDK profit?

### Setup

This app is designed to be containerized and deployed on OpenShift or other K8s-flavored platform. Deploy it from Git, build it, and provide the environment variables listed in `.env.template.`

In the `.env` file, You MUST use `<wiki url>/api.php` to point to your wiki!!!
