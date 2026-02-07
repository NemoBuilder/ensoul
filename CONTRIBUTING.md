# Contributing to Ensoul

Thank you for your interest in contributing to Ensoul! This document provides guidelines for contributing to the project.

## Getting Started

### Prerequisites

- **Go 1.21+** — Backend development
- **Node.js 18+** — Frontend development
- **PostgreSQL 15+** — Database
- **Git** — Version control

### Local Development Setup

1. **Clone the repository**
   ```bash
   git clone https://github.com/ensoul-labs/ensoul.git
   cd ensoul
   ```

2. **Backend setup**
   ```bash
   cd server
   cp .env.example .env
   # Edit .env with your local database URL, API keys, etc.
   go run main.go
   ```

3. **Frontend setup**
   ```bash
   cd web
   npm install
   npm run dev
   ```

4. **Database** — Ensure PostgreSQL is running. Tables are auto-migrated on first start.

## Project Structure

```
ensoul/
├── server/           # Go backend
│   ├── chain/        # BNB Chain + ERC-8004 interaction
│   ├── contracts/    # Contract ABI bindings
│   ├── services/     # Business logic + AI layer
│   ├── handlers/     # HTTP route handlers
│   ├── middleware/    # Authentication middleware
│   ├── models/       # GORM database models
│   ├── config/       # Environment configuration
│   ├── database/     # Database connection
│   ├── router/       # Route definitions
│   └── cmd/          # CLI tools and tests
├── web/              # Next.js frontend
│   ├── src/app/      # Page routes
│   ├── src/components/ # UI components
│   └── src/lib/      # API client + utilities
├── skills/           # OpenClaw Skill files
├── deploy/           # Deployment configuration
└── docs/             # Documentation
```

## Code Standards

### General

- **Language**: All code comments, UI text, error messages, and documentation must be in **English**
- **Commits**: Use clear, descriptive commit messages

### Go Backend

- Follow standard Go conventions and `gofmt` formatting
- Keep handler functions thin — business logic goes in `services/`
- Use structured error handling (return errors, don't panic)
- Add comments for exported functions and types

### TypeScript Frontend

- Follow the existing TailwindCSS styling patterns
- Use the brand color palette defined in `globals.css`
- Components go in `src/components/`, pages in `src/app/`
- Use the API client in `src/lib/api.ts` for all backend calls

## How to Contribute

### Reporting Issues

- Use GitHub Issues
- Include steps to reproduce, expected behavior, and actual behavior
- Attach screenshots for UI issues

### Submitting Changes

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes
4. Test locally (both backend and frontend)
5. Commit with clear messages
6. Push to your fork
7. Open a Pull Request against `main`

### Areas of Contribution

- **Bug fixes** — Always welcome
- **UI improvements** — Animations, responsive design, accessibility
- **New dimensions** — Expanding beyond the current six personality dimensions
- **Agent integration** — Support for more AI agent frameworks beyond OpenClaw
- **Chain features** — Revenue distribution, governance, multi-chain support
- **Documentation** — Tutorials, API examples, guides

## Testing

### Backend
```bash
cd server
go build ./...            # Compile check
go test ./...             # Unit tests
go run cmd/test_e2e/main.go  # End-to-end test
```

### Frontend
```bash
cd web
npm run build             # Build check
npm run lint              # Lint check
```

## License

By contributing to Ensoul, you agree that your contributions will be licensed under the [MIT License](LICENSE).
