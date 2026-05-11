# AgentGrid — Note di sviluppo

AgentGrid è un progetto open-source che permette di deployare un SaaS di agenti schedulati su una VPS con Docker.

## Funzionamento generale

1. L'utente fa login con GitHub (modalità public) o entra direttamente (modalità local).
2. Dal pannello può creare **progetti**.
3. Dentro ogni progetto può creare **agenti**.
4. Per ogni agente si definisce:
   - Un **cron expression** (quando svegliarsi)
   - Un **prompt** (cosa fare al risveglio)
   - Un'immagine Docker oppure un **Dockerfile** personalizzato
   - Una **working directory** nel container
5. Lo scheduler si occupa di eseguire l'agente quando scatta il cron.
6. Ogni esecuzione viene loggata nella tab **Logs** e riassunta nella tab **Sessions**.

## Modalità di deploy

- **Local** (`AGENTGRID_MODE=local`): nessuna autenticazione, utente locale creato automaticamente.
- **Public** (`AGENTGRID_MODE=public`): richiede GitHub OAuth.

## Azioni manuali

- **Run Now**: esegue immediatamente l'agente (build + run).
- **Dry Run**: builda solo l'immagine senza eseguire il prompt.

## Architettura

- `main.go`: entry point, carica `.env`, avvia DB, scheduler e server.
- `internal/db`: SQLite con migrazioni automatiche.
- `internal/scheduler`: cron con robfig/cron, gestisce build e run Docker.
- `internal/docker`: wrapper per `docker build`, `docker run` e `docker pull`.
- `internal/server`: router Chi, middleware auth, handler HTML.
- `templates/`: HTML con TailwindCSS.

## Roadmap

- [x] Login GitHub OAuth
- [x] Progetti e agenti
- [x] Scheduling cron
- [x] Supporto Dockerfile per build custom
- [x] Run Now e Dry Run
- [x] Tab Logs e Sessions
- [x] Modalità local senza auth
- [ ] WebSocket per log in tempo reale
- [ ] Notifiche (email, Slack) su fallimenti
- [ ] Multi-utente avanzato (ruoli, permessi)
