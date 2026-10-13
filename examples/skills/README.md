# Skills examples

This directory holds examples for the Agent Skills feature.

## Layout

```text
skills/
├── README.md              # this file
└── pdf-processing/        # example PDF processing skill
    ├── SKILL.md           # main file (Level 2)
    ├── FORMS.md           # supplementary doc (Level 3)
    └── scripts/           # executable scripts
        ├── analyze_form.py
        └── extract_text.py
```

## Quick start

### Run the demo

```bash
go run ./cmd/skills-demo/main.go
```

### Create a new skill

1. Create a folder in this directory:

   ```bash
   mkdir my-new-skill
   ```

2. Create `SKILL.md`:

   ```markdown
   ---
   name: my-new-skill
   description: Description of what this skill does and when to use it.
   ---

   # My New Skill

   Instructions for the agent...
   ```

3. Add scripts (optional):

   ```bash
   mkdir my-new-skill/scripts
   # add your scripts
   ```

## Full documentation

See the [Agent Skills docs](../../website-docs/03-features/22-skills-sandbox.md).

## Example: pdf-processing

A complete example skill that shows:

- **SKILL.md**: the main file with YAML frontmatter
- **FORMS.md**: a supplementary reference doc
- **scripts/**: Python scripts that run in the sandbox

### Skill description

```yaml
name: pdf-processing
description: Extract text and tables from PDF files, fill forms, merge documents.
```

### Included scripts

| Script | Purpose |
|------|------|
| `analyze_form.py` | Analyze PDF form fields |
| `extract_text.py` | Extract text from a PDF |

### Usage example

The agent calls the skill on its own, based on the user's request:

```text
User: "What fields does this PDF form have?"

Agent:
  1. Matches the pdf-processing skill
  2. Calls read_file(path="skill://pdf-processing/SKILL.md") to load the skill
  3. Calls shell_exec(skill_name="pdf-processing", command=...) to run analyze_form.py
  4. Returns the form field analysis
```
