FROM mcr.microsoft.com/devcontainers/base:ubuntu-22.04

USER vscode
ENV HOME=/home/vscode

# Install Git AI (track AI-generated code in Git)
RUN curl -sSL https://usegitai.com/install.sh | bash

# Install Cursor CLI
RUN curl -fsSL https://cursor.com/install | bash

# Expose vscode's bin dirs on PATH for all users/shells
USER root
RUN echo 'export PATH="/home/vscode/.git-ai/bin:/home/vscode/.local/bin:$PATH"' > /etc/profile.d/git-ai-cursor.sh \
    && chmod 644 /etc/profile.d/git-ai-cursor.sh

USER vscode
