# AgentGrid

AgentGrid is a lightweight, self-hosted platform for scheduling and running containerised agents on your own infrastructure. Think of it as a personal cron-as-a-service with a web dashboard and Docker support.

## Features

- **Projects** — Organise agents into projects.
- **Agents** — Each agent is a scheduled Docker container with a custom prompt.
- **Cron Scheduling** — Define when agents wake up using standard cron expressions.
- **Custom Dockerfiles** — Build images on the fly from a per-agent Dockerfile, or use a pre-built image.
- **Dry Run** — Build the image without executing the prompt, perfect for verifying your Dockerfile.
- **Run Now** — Trigger an agent immediately from the dashboard.
- **Logs & Sessions** — Inspect every execution, its output, start/finish times and status.
- **Two Deployment Modes** — Run locally without authentication, or deploy publicly with GitHub OAuth.

## Quick Start

```bash
# Clone the repository
git clone https://github.com/yourusername/agentgrid.git
cd agentgrid

# Copy the environment file
cp .env.example .env

# Start the server (local mode — no auth required)
make start
```

Open http://localhost:8080 and you are ready to create your first project and agent.

## Deployment Modes

AgentGrid supports two modes controlled by the `AGENTGRID_MODE` environment variable.

### Local Mode (default)

Ideal for running on your own machine. No authentication is required; a default local user is created automatically.

```bash
AGENTGRID_MODE=local
```

### Public Mode

For deployments on a public VPS. Users must sign in with GitHub OAuth before accessing the dashboard.

```bash
AGENTGRID_MODE=public
GITHUB_CLIENT_ID=your_github_client_id
GITHUB_CLIENT_SECRET=your_github_client_secret
BASE_URL=https://your-domain.com
```

## Requirements

- Go 1.22+
- Docker (the host must have a running Docker daemon)
- SQLite (embedded, no extra setup needed)

## Configuration

All configuration is done through environment variables or a `.env` file.

| Variable | Description | Default |
|----------|-------------|---------|
| `AGENTGRID_MODE` | `local` or `public` | `public` |
| `GITHUB_CLIENT_ID` | GitHub OAuth client ID | — |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth client secret | — |
| `BASE_URL` | Base URL for OAuth callbacks | `http://localhost:8080` |
| `PORT` | HTTP server port | `8080` |
| `DATABASE_PATH` | SQLite database file path | `data/agentgrid.db` |

## Project Structure

```
.
├── main.go                  # Application entry point
├── Makefile                 # Common tasks (start, build, clean)
├── .env.example             # Example environment file
├── internal/
│   ├── db/                  # SQLite database and migrations
│   ├── models/              # Data models
│   ├── scheduler/           # Cron scheduler for agents
│   ├── docker/              # Docker build and run helpers
│   └── server/              # HTTP handlers, auth, middleware
├── templates/               # HTML templates
├── static/                  # Static assets
└── docs/
    └── features.md          # Detailed feature documentation
```

## Agent Lifecycle

1. **Create** an agent inside a project with a name, cron expression, prompt and optional Dockerfile.
2. **Schedule** — The scheduler automatically registers active agents with the cron runner.
3. **Wake Up** — When the cron expression fires, the agent builds its image (if a Dockerfile is provided), runs the prompt inside a container and records the output.
4. **Inspect** — View logs and sessions in the dashboard tabs.

## API / Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Redirect to dashboard or login |
| GET | `/login` | Login page (public mode) |
| GET | `/auth/github` | Initiate GitHub OAuth |
| GET | `/auth/github/callback` | OAuth callback |
| GET | `/auth/local` | Local mode auto-login |
| POST | `/auth/logout` | Clear session |
| GET | `/dashboard` | Dashboard |
| GET / POST | `/projects` | List / create projects |
| GET | `/projects/{id}` | View project |
| POST | `/projects/{id}/delete` | Delete project |
| POST | `/projects/{id}/agents` | Create agent |
| GET | `/agents/{id}` | View / configure agent |
| POST | `/agents/{id}` | Update agent |
| POST | `/agents/{id}/delete` | Delete agent |
| POST | `/agents/{id}/run` | Run agent now |
| POST | `/agents/{id}/dry-run` | Dry run (build image only) |
| GET | `/agents/{id}/runs` | Agent runs (HTML) |

## License

MIT
