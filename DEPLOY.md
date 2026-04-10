# 1. Обновление системы и установка зависимостей
sudo apt update
sudo apt install -y ca-certificates curl gnupg lsb-release

# 2. Добавление официального GPG-ключа Docker
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

# 3. Добавление репозитория Docker в список источников APT
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# 4. Установка Docker Engine и Docker Compose (как плагин)
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin


# Подготовка релиза
```bash
GOLANG_BASE_REF="$(awk -F= '/^GOLANG_1_26_ALPINE_3_23_IMAGE=/{print $2}' compliance/base-images.lock)"
ALPINE_BASE_REF="$(awk -F= '/^ALPINE_3_23_IMAGE=/{print $2}' compliance/base-images.lock)"
docker build --platform linux/amd64 \
  --build-arg GOLANG_BASE_REF="$GOLANG_BASE_REF" \
  --build-arg ALPINE_BASE_REF="$ALPINE_BASE_REF" \
  -t registry.gitjet.ru/core-scm:v0.0.1-linux-amd64 \
  .
```