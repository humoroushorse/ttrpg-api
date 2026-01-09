# Architecture Diagrams

This directory contains visual documentation for the Go Sprint Management System using Mermaid diagram syntax.

## Diagram Types

### Entity Relationship Diagrams (ERD)
- **[database-erd.md](database-erd.md)**: Complete database schema with relationships

### C4 Architecture Diagrams
- **[c4-system-context.md](c4-system-context.md)**: System context showing external actors and systems
- **[c4-container.md](c4-container.md)**: Container diagram showing major components
- **[c4-component.md](c4-component.md)**: Component diagram showing internal structure

### Sequence Diagrams
- **[sequence-work-item-creation.md](sequence-work-item-creation.md)**: Work item creation flow
- **[sequence-sprint-closure.md](sequence-sprint-closure.md)**: Sprint closure workflow
- **[sequence-nats-messaging.md](sequence-nats-messaging.md)**: NATS message flows
- **[sequence-websocket-notification.md](sequence-websocket-notification.md)**: Real-time notification flow
- **[sequence-authentication.md](sequence-authentication.md)**: Authentication flow

### User Flow Diagrams
- **[user-flow-sprint-planning.md](user-flow-sprint-planning.md)**: Sprint planning workflow
- **[user-flow-work-item-lifecycle.md](user-flow-work-item-lifecycle.md)**: Work item lifecycle
- **[user-flow-dependency-management.md](user-flow-dependency-management.md)**: Dependency management

### Deployment Diagrams
- **[deployment-kubernetes.md](deployment-kubernetes.md)**: Kubernetes deployment architecture
- **[deployment-docker.md](deployment-docker.md)**: Docker deployment architecture

## Viewing Diagrams

### In GitHub
GitHub automatically renders Mermaid diagrams in markdown files.

### In VS Code
Install the "Markdown Preview Mermaid Support" extension.

### Online
Copy diagram code to [Mermaid Live Editor](https://mermaid.live/)

## Diagram Conventions

- **Blue boxes**: Internal services/components
- **Green boxes**: External services/systems
- **Orange boxes**: Data stores
- **Purple boxes**: Message brokers
- **Solid lines**: Synchronous communication
- **Dashed lines**: Asynchronous communication
- **Arrows**: Data flow direction
