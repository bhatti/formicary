FROM ubuntu:24.04

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# System tools required by AI agent scripts
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    curl \
    jq \
    git \
    xxd \
    ca-certificates \
    gnupg \
    && rm -rf /var/lib/apt/lists/*

# GitHub CLI
RUN curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
    | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
    | tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
    && apt-get update \
    && apt-get install -y --no-install-recommends gh \
    && rm -rf /var/lib/apt/lists/*

# Node.js 22.x (for Claude Code CLI)
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# Claude Code CLI — pinned for reproducibility; update when upgrading
RUN npm install -g @anthropic-ai/claude-code@1

# you-got-skills: optional skill slash-commands for Claude
# Install to the system skill directory so all users have access
RUN git clone https://github.com/bhatti/you-got-skills.git /opt/you-got-skills \
    && cd /opt/you-got-skills \
    && ./setup \
    # Make skills available system-wide for the non-root user Claude runs as
    && mkdir -p /root/.claude/skills \
    && ln -s /opt/you-got-skills /root/.claude/skills/you-got-skills

# Git identity for commits made by the AI agent inside the container
RUN git config --global user.email "ai-agent@formicary" \
    && git config --global user.name "Formicary AI Agent" \
    # Allow git operations in any directory (container mounts /workspace)
    && git config --global --add safe.directory '*'

# Run as non-root to reduce blast radius from AI-generated shell commands
RUN useradd -m -u 1001 agent \
    && mkdir -p /workspace /home/agent/.claude/skills \
    && ln -s /opt/you-got-skills /home/agent/.claude/skills/you-got-skills \
    && chown -R agent:agent /workspace /home/agent

USER agent

# Default working directory; Formicary writes artifacts here
WORKDIR /workspace
