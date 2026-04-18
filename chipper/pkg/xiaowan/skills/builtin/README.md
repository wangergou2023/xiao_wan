# Builtin Skills

These are real builtin skill files bundled with XiaoWan.

- `vector_companion`: default physical robot companion behavior, auto enabled
- `owner_mode`: stronger owner-aware tone and relationship memory
- `home_companion`: natural home chat and daily assistance
- `joke_mode`: short spoken jokes and playful replies
- `story_teller`: bedtime / short story speaking style
- `cooking_helper`: concise cooking guidance for voice interaction

You can add more skills later by creating:

- `skills/<skill-name>/SKILL.md`
- `chipper/skills/<skill-name>/SKILL.md`

Each skill uses front matter like:

```md
---
name: my_skill
description: What this skill does.
auto_use: false
---
Prompt content here.
```
