FROM mcr.microsoft.com/devcontainers/base:ubuntu-22.04

USER vscode
ENV HOME=/home/vscode

# Remote install scripts: dev container only. Scripts from trusted sources
# (usegitai.com, cursor.com). No checksum verification available from upstream;
# risk accepted for development environment. Do not use in production images.

# Install Git AI (dev container only; script from usegitai.com)
RUN curl -sSL -o /tmp/git-ai-install.sh https://usegitai.com/install.sh && \
    bash /tmp/git-ai-install.sh && rm /tmp/git-ai-install.sh

# Install Cursor CLI (dev container only; script from cursor.com)
RUN curl -fsSL -o /tmp/cursor-install.sh https://cursor.com/install && \
    bash /tmp/cursor-install.sh && rm /tmp/cursor-install.sh

# Expose vscode's bin dirs on PATH for all users/shells
USER root
RUN echo 'export PATH="/home/vscode/.git-ai/bin:/home/vscode/.local/bin:$PATH"' > /etc/profile.d/git-ai-cursor.sh \
    && chmod 644 /etc/profile.d/git-ai-cursor.sh

USER vscode

# Dedicated npm cache owned by vscode; prevents EACCES when Node feature runs as root
ENV NPM_CONFIG_CACHE=/home/vscode/.npm-cache
RUN mkdir -p /home/vscode/.npm-cache
