# ==========================================
# Dockerfile для поднятия code-server (VS Code в браузере)
# на базе Ubuntu 24.04 с поддержкой Python и ML
# ==========================================

FROM ubuntu:24.04

# Отключаем интерактивные запросы при установке пакетов
ENV DEBIAN_FRONTEND=noninteractive

# Обновляем и ставим базовый софт + Python + ML зависимости
RUN apt-get update && apt-get install -y \
    curl \
    sudo \
    git \
    wget \
    build-essential \
    python3 \
    python3-pip \
    python3-venv \
    nodejs \
    npm \
    htop \
    tmux \
    vim \
    nano \
    unzip \
    && rm -rf /var/lib/apt/lists/*

# Устанавливаем code-server
RUN curl -fsSL https://code-server.dev/install.sh | sh

# Создаём пользователя
RUN useradd -m -u 1000 ubuntu && \
    echo "ubuntu ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers && \
    mkdir -p /home/ubuntu/workspace && \
    chown -R ubuntu:ubuntu /home/ubuntu

# Переключаемся на пользователя
USER ubuntu
WORKDIR /home/ubuntu/workspace

# Устанавливаем Python пакеты для ML
RUN pip3 install --user --no-cache-dir \
    numpy \
    pandas \
    matplotlib \
    seaborn \
    scikit-learn \
    jupyter \
    notebook \
    ipykernel \
    transformers \
    datasets \
    torch \
    torchvision \
    torchaudio \
    --index-url https://download.pytorch.org/whl/cpu

# Устанавливаем дополнительные пакеты
RUN pip3 install --user --no-cache-dir \
    fastapi \
    uvicorn \
    gradio \
    streamlit \
    requests \
    pillow \
    tqdm

# Открываем порт 7860
EXPOSE 7860

# Запускаем code-server
CMD ["code-server", "--bind-addr", "0.0.0.0:7860", "--auth", "password"]
