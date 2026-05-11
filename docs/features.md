# AgentGrid Features

## Deployment Modes

AgentGrid supports two deployment modes controlled by the `AGENTGRID_MODE` environment variable:

- **local** — No authentication required. Ideal for personal deployments on your own machine. The application automatically provisions a local user so you can start using it immediately.
- **public** — GitHub OAuth authentication is required. Intended for deployments on a public VPS. Users must sign in with GitHub before accessing projects and agents.

## Projects

- Users can create multiple projects.
- Each project has a name and an optional description.
- Projects are isolated: agents belong to a single project.
- Deleting a project removes all associated agents and their run history (cascaded by the database).

## Agents

An agent is a scheduled unit of work inside a project.

### Configuration

The agent configuration page is organised in tabs:

1. **Wake Up**
   - **Cron expression** — A standard cron string that defines when the agent wakes up and executes.
   - **Wake up prompt** — The prompt/command that is passed to the agent at wake-up time.

2. **Docker**
   - **Docker image** — The base image used to run the agent container.
   - **Working directory** — The directory inside the container where the command runs.
   - **Dockerfile** — An optional custom Dockerfile. When provided, AgentGrid builds the image from this Dockerfile instead of pulling a pre-built image.

3. **Logs**
   - Shows the full output of every executed run.
   - Displays status (`pending`, `running`, `completed`, `failed`, `dry-run`), timestamps, and stdout/stderr.

4. **Sessions**
   - A historical list of every time the agent woke up.
   - Each row shows the run ID, status, start/finish times, and a summary of what the agent did.

### Actions

From the top of the agent page you can trigger two immediate actions:

- **Run Now** — Executes the agent immediately and records the result as a normal run.
- **Dry Run** — Executes the agent immediately but records the result with status `dry-run` so it is clearly marked as a test.

Both actions redirect to the **Logs** tab after completion so you can inspect the output right away.

## Scheduling

- Agents are scheduled using a cron expression.
- Only active agents are scheduled.
- The scheduler starts automatically when the application boots and loads all active agents from the database.
- When an agent is updated, its schedule is removed and re-added with the new expression.
- When an agent is deleted, its schedule is removed.

## Database

- SQLite is used as the default database.
- The path is configurable via `DATABASE_PATH`.
- Migrations run automatically on startup.

## Authentication (public mode)

- GitHub OAuth 2.0.
- Requires `GITHUB_CLIENT_ID` and `GITHUB_CLIENT_SECRET`.
- Sessions are stored as HTTP-only cookies.

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `AGENTGRID_MODE` | `local` or `public` | `public` |
| `GITHUB_CLIENT_ID` | GitHub OAuth client ID | — |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth client secret | — |
| `BASE_URL` | Base URL for OAuth callbacks | `http://localhost:8080` |
| `PORT` | HTTP server port | `8080` |
| `DATABASE_PATH` | SQLite database file path | `data/agentgrid.db` |
