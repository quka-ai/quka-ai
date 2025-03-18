<p align="center">
 <img align="center" src="https://raw.githubusercontent.com/breeew/brew/main/assets/logo.png" height="96" />
 <h1 align="center">
  Brew
 </h1>
</p>

![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/breeew/brew/ghcr.yml)
![GitHub Tag](https://img.shields.io/github/v/tag/breeew/brew)
![GitHub commit activity](https://img.shields.io/github/commit-activity/m/breeew/brew)

![Discord](https://img.shields.io/discord/1293497229096521768?logo=discord&logoColor=white)
![X (formerly Twitter) Follow](https://img.shields.io/twitter/follow/trybrewapp)

**Brew** is a lightweight and user-friendly Retrieval-Augmented Generation (RAG) system designed to help you build your own second brain. With its fast and easy-to-use interface, Brew empowers users to efficiently manage and retrieve information.


https://github.com/user-attachments/assets/78751333-e02f-482d-b2da-12d0ea59a66c


## Community

Join our community on Discord to connect with other users, share ideas, and get support: [Discord Community](https://discord.gg/YGrbmbCVRF).

## Install

### Databases

- Install DB: [pgvector](https://github.com/pgvector/pgvector)，don't forget `CREATE EXTENSION vector;`
- Create database like 'brew'
- Execute create table sqls via `/internal/store/sqlstore/*.sql`

### Service

- Clone & go build cmd/main.go
- Copy default config(cmd/service/etc/service-default.toml) to your config path  
  `brew-api service -c {your config path}` to start selfhost service.

### Web

- [brew web-app](https://github.com/breeew/web-app)
