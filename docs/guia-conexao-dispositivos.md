# Guia de Conexão de Dispositivos

Como conectar um dispositivo IoT (ESP32, Arduino, Raspberry Pi, etc.) à API de armazenamento de dados.

Existem duas opções: **REST (HTTP)** e **MQTT**. Ambas aceitam o mesmo formato de payload JSON e armazenam os dados da mesma forma no banco.

---

## Deploy e Limitações

| Ambiente | REST | MQTT | Observação |
|----------|------|------|------------|
| **Render / Cloud (HTTPS)** | Sim | Não | Render expõe apenas portas HTTP/HTTPS. O broker MQTT (porta 1883 TCP) não é acessível externamente. Use REST. |
| **Local / VPS / Docker** | Sim | Sim | Ambos funcionam. MQTT na porta 1883, REST na porta configurada. |

**URL de produção (Render):** `https://go-pe.onrender.com`

---

## Obtendo Credenciais

Antes de conectar um dispositivo, você precisa de credenciais de autenticação.

### Opção A: API Key Global (mais simples para REST)

A API Key é uma chave única compartilhada entre todos os dispositivos. Basta incluir no header `X-API-Key` das requisições REST.

- A chave está definida na variável de ambiente `DEVICE_API_KEY` no servidor
- No Render: acesse o painel > seu serviço > **Environment** > copie o valor de `DEVICE_API_KEY`
- Localmente: definida no arquivo `.env`

Com a API Key, dispositivos são criados automaticamente no primeiro envio (se `DEVICE_AUTO_CREATE=true`). Não precisa cadastrar o dispositivo antes.

### Opção B: Token Individual por Dispositivo (REST e MQTT)

Cada dispositivo recebe um token único ao ser registrado. Necessário para MQTT e também funciona como alternativa à API Key no REST. Mais seguro — se um dispositivo for comprometido, os outros não são afetados.

**Pelo Dashboard (recomendado):**

1. Acesse o dashboard web e faça login
2. Vá na aba **Dispositivos**
3. Clique em **Adicionar Dispositivo**
4. Preencha nome, tipo e localização
5. Após criar, um dialog aparece com o **Auth Token** — **copie e salve imediatamente**, ele não será exibido novamente com destaque
6. Na tabela de dispositivos, a coluna **Auth Token** mostra os primeiros caracteres com um botão de copiar para obter o token completo

**Pela API:**

```bash
# Primeiro, faça login para obter o JWT
curl -X POST https://go-pe.onrender.com/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"seu-email@example.com","password":"sua-senha"}'

# Use o JWT para registrar o dispositivo
curl -X POST https://go-pe.onrender.com/auth/register-device \
  -H "Authorization: Bearer <jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "esp32-sala",
    "description": "Sensor de temperatura da sala",
    "device_type": "sensor",
    "location": "Sala"
  }'
```

A resposta inclui o `auth_token` do dispositivo:

```json
{
  "device": {
    "id": 5,
    "name": "esp32-sala",
    "auth_token": "r9GahY_64Uy8rnW0DCN9OAo9-aAxhIsxhmP43QuPCpo=",
    "is_active": true
  },
  "auth_token": "r9GahY_64Uy8rnW0DCN9OAo9-aAxhIsxhmP43QuPCpo="
}
```

---

## Formato do Payload

Voce pode usar **nomes personalizados** para os campos (sinais):

```json
{
  "device_id": "meu-sensor-01",
  "temperatura": 23.5,
  "umidade": 65,
  "motor_ligado": true,
  "status": "tudo-ok"
}
```

O formato legado com `field_1` a `field_8` tambem continua funcionando:

```json
{
  "device_id": "meu-sensor-01",
  "field_1": 23.5,
  "field_2": 65
}
```

- `device_id` (obrigatorio): **nome** do dispositivo (nao o ID numerico do banco)
- Qualquer outra chave no JSON e tratada como um sinal (ex: `"temperatura"`, `"abobora"`, `"nivel_rio"`)
- **Correspondencia de nomes**: a comparacao e feita em minusculas e sem espacos, entao `"Temperatura"`, `"temperatura"` e `"Temperatura Sala"` vs `"temperaturasala"` correspondem ao mesmo sinal
- Tipos de valores:
  - **Numero** -> salvo como sinal analogico
  - **Booleano** -> salvo como sinal digital
  - **String** -> salvo nos metadados
- Dispositivos e sinais sao criados automaticamente no primeiro envio (se usando API Key com `DEVICE_AUTO_CREATE=true`)

---

## Opcao 1: REST (HTTP POST)

O endpoint e `POST /devices/data`. Funciona em qualquer ambiente (Render, local, VPS).

### Autenticacao

Escolha **uma** das opcoes:

| Metodo | Header | Quando usar |
|--------|--------|-------------|
| API Key | `X-API-Key: <sua-api-key>` | Mais simples, dispositivo criado automaticamente |
| Token do dispositivo | `Authorization: Bearer <token>` | Dispositivo ja registrado, mais seguro por dispositivo |

### Exemplo com cURL

```bash
# Com API Key (mais simples - dispositivo criado automaticamente)
curl -X POST https://go-pe.onrender.com/devices/data \
  -H "Content-Type: application/json" \
  -H "X-API-Key: <sua-api-key>" \
  -d '{
    "device_id": "sensor-temperatura-01",
    "temperatura": 24.3,
    "umidade": 61.5
  }'

# Com token do dispositivo (registrado previamente)
curl -X POST https://go-pe.onrender.com/devices/data \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token-do-dispositivo>" \
  -d '{
    "device_id": "sensor-temperatura-01",
    "temperatura": 24.3,
    "umidade": 61.5,
    "motor_ligado": true
  }'
```

Resposta de sucesso: `201 Created` com `{"status": "ok"}`.

### Exemplo com ESP32 (Arduino + ArduinoJson)

Exemplo completo com reconexao WiFi, retry, e suporte a ambos os modos de autenticacao.

```cpp
#include <WiFi.h>
#include <HTTPClient.h>
#include <ArduinoJson.h>

// ============================================================
// CONFIGURACAO - Altere estes valores
// ============================================================

// WiFi
const char* WIFI_SSID     = "SUA_REDE";
const char* WIFI_PASSWORD  = "SUA_SENHA";

// Backend
const char* SERVER_URL = "https://go-pe.onrender.com/devices/data";

// Autenticacao: escolha UMA opcao, comente a outra.
// Opcao A: API Key global (dispositivo criado automaticamente)
// const char* API_KEY    = "SUA_API_KEY";  // de Render > Environment > DEVICE_API_KEY
// const char* AUTH_TOKEN = NULL;

// Opcao B: Token individual do dispositivo (mais seguro)
const char* API_KEY    = NULL;
const char* AUTH_TOKEN = "TOKEN_DO_DISPOSITIVO";  // de POST /auth/register-device

// Identidade do dispositivo
const char* DEVICE_ID = "esp32-sala";

// Tempo entre envios (ms)
const unsigned long SEND_INTERVAL_MS = 60000;  // 60 segundos
const int MAX_RETRIES = 3;

// ============================================================
// GLOBAIS
// ============================================================
unsigned long lastSendTime = 0;
int failCount = 0;

void setup() {
  Serial.begin(115200);
  delay(1000);
  pinMode(2, OUTPUT);  // LED built-in

  Serial.println("\n================================");
  Serial.printf("Dispositivo: %s\n", DEVICE_ID);
  Serial.printf("Servidor: %s\n", SERVER_URL);
  Serial.println("================================\n");

  connectWiFi();
  sendSensorData();
  lastSendTime = millis();
}

void loop() {
  if (WiFi.status() != WL_CONNECTED) {
    connectWiFi();
  }

  if (millis() - lastSendTime >= SEND_INTERVAL_MS) {
    sendSensorData();
    lastSendTime = millis();
  }

  delay(100);
}

// ============================================================
// Conexao WiFi
// ============================================================
void connectWiFi() {
  Serial.printf("[WIFI] Conectando a %s", WIFI_SSID);
  WiFi.mode(WIFI_STA);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);

  unsigned long start = millis();
  while (WiFi.status() != WL_CONNECTED && millis() - start < 15000) {
    Serial.print(".");
    delay(500);
  }

  if (WiFi.status() == WL_CONNECTED) {
    Serial.printf("\n[WIFI] Conectado! IP: %s\n", WiFi.localIP().toString().c_str());
    digitalWrite(2, HIGH);
  } else {
    Serial.println("\n[WIFI] Falha na conexao.");
    digitalWrite(2, LOW);
  }
}

// ============================================================
// Leitura dos sensores e envio
// ============================================================
void sendSensorData() {
  if (WiFi.status() != WL_CONNECTED) return;

  // Substitua pelas leituras reais dos seus sensores:
  float temperatura = 23.5;
  float umidade     = 60.0;
  bool  movimento   = false;

  // Monta o JSON — use nomes descritivos para os campos
  JsonDocument doc;
  doc["device_id"]    = DEVICE_ID;
  doc["temperatura"]  = temperatura;
  doc["umidade"]      = umidade;
  doc["movimento"]    = movimento;
  // doc["nivel_rio"] = outroValor;  // adicione mais campos conforme necessario

  String payload;
  serializeJson(doc, payload);
  Serial.printf("[ENVIO] %s\n", payload.c_str());

  // Envia com retry
  bool success = false;
  for (int attempt = 1; attempt <= MAX_RETRIES && !success; attempt++) {
    if (attempt > 1) {
      Serial.printf("[ENVIO] Tentativa %d/%d...\n", attempt, MAX_RETRIES);
      delay(5000);
    }
    success = httpPost(payload);
  }

  if (success) {
    failCount = 0;
  } else {
    failCount++;
    if (failCount >= 5) {
      Serial.println("[ENVIO] Muitas falhas, reconectando WiFi...");
      WiFi.disconnect();
      delay(1000);
      connectWiFi();
      failCount = 0;
    }
  }
}

// ============================================================
// HTTP POST
// ============================================================
bool httpPost(const String& payload) {
  HTTPClient http;
  http.begin(SERVER_URL);
  http.addHeader("Content-Type", "application/json");

  // Autenticacao
  if (API_KEY != NULL) {
    http.addHeader("X-API-Key", API_KEY);
  } else if (AUTH_TOKEN != NULL) {
    http.addHeader("Authorization", String("Bearer ") + AUTH_TOKEN);
  }

  http.setTimeout(10000);
  int httpCode = http.POST(payload);
  String response = http.getString();
  http.end();

  if (httpCode == 201) {
    Serial.printf("[HTTP] OK (201) - %s\n", response.c_str());
    return true;
  } else if (httpCode > 0) {
    Serial.printf("[HTTP] Erro %d - %s\n", httpCode, response.c_str());
    return false;
  } else {
    Serial.printf("[HTTP] Falha na conexao: %s\n", http.errorToString(httpCode).c_str());
    return false;
  }
}
```

**Bibliotecas necessarias no Arduino IDE:**
- `ArduinoJson` (por Benoit Blanchon) - instale via Library Manager

### Exemplo com MicroPython (ESP32/ESP8266)

```python
import urequests
import ujson
import time
import network

# Configuracao
WIFI_SSID = "SUA_REDE"
WIFI_PASS = "SUA_SENHA"
SERVER_URL = "https://go-pe.onrender.com/devices/data"
DEVICE_ID = "esp32-sala"

# Autenticacao: escolha uma opcao
API_KEY = "SUA_API_KEY"        # Opcao A: API Key global
# AUTH_TOKEN = "TOKEN_DO_DEVICE"  # Opcao B: Token individual

def connect_wifi():
    wlan = network.WLAN(network.STA_IF)
    wlan.active(True)
    if not wlan.isconnected():
        print("Conectando WiFi...")
        wlan.connect(WIFI_SSID, WIFI_PASS)
        while not wlan.isconnected():
            time.sleep(0.5)
    print("WiFi conectado:", wlan.ifconfig()[0])

def enviar_dados(temperatura, umidade):
    payload = ujson.dumps({
        "device_id": DEVICE_ID,
        "temperatura": temperatura,
        "umidade": umidade
    })

    headers = {"Content-Type": "application/json"}

    # Escolha o header de autenticacao
    if "API_KEY" in dir():
        headers["X-API-Key"] = API_KEY
    else:
        headers["Authorization"] = "Bearer " + AUTH_TOKEN

    try:
        response = urequests.post(SERVER_URL, data=payload, headers=headers)
        print("Status:", response.status_code, response.text)
        response.close()
    except Exception as e:
        print("Erro:", e)

connect_wifi()
while True:
    temp = 23.5  # substituir pela leitura real
    hum = 60.0
    enviar_dados(temp, hum)
    time.sleep(60)
```

### Exemplo com Python (Raspberry Pi / PC)

```python
import requests
import json
import time

SERVER_URL = "https://go-pe.onrender.com/devices/data"
DEVICE_ID = "raspberry-pi-01"

# Autenticacao: escolha uma opcao
API_KEY = "SUA_API_KEY"
# AUTH_TOKEN = "TOKEN_DO_DEVICE"

def enviar_dados(temperatura, umidade):
    payload = {
        "device_id": DEVICE_ID,
        "temperatura": temperatura,
        "umidade": umidade
    }

    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["X-API-Key"] = API_KEY
    else:
        headers["Authorization"] = f"Bearer {AUTH_TOKEN}"

    response = requests.post(SERVER_URL, json=payload, headers=headers)
    print(f"Status: {response.status_code} - {response.text}")

while True:
    enviar_dados(23.5, 60.0)  # substituir por leituras reais
    time.sleep(60)
```

---

## Opcao 2: MQTT

O servidor possui um broker MQTT embutido (mochi-mqtt). O dispositivo conecta via MQTT e publica mensagens no topico `devices/{nome-do-dispositivo}/data`.

> **Importante:** MQTT so funciona em deploys locais ou VPS onde a porta TCP 1883 esta acessivel. No Render e plataformas similares (que so expoem HTTP/HTTPS), use REST.

### Pre-requisitos

O broker precisa estar habilitado no servidor:

```env
MQTT_BROKER_ENABLED=true
MQTT_BROKER_PORT=1883
```

### Autenticacao MQTT

- **Username**: nome do dispositivo (mesmo valor de `name` no banco)
- **Password**: `auth_token` do dispositivo

O dispositivo precisa estar cadastrado previamente. Nao ha auto-criacao via MQTT. Registre pelo **dashboard** (aba Dispositivos -> Adicionar Dispositivo) ou pela API conforme descrito na secao [Obtendo Credenciais](#obtendo-credenciais).

### Topico

O dispositivo so pode publicar no seu proprio topico:

```
devices/{nome-do-dispositivo}/data
```

Exemplo: dispositivo `esp32-sala` publica em `devices/esp32-sala/data`.

### Exemplo com ESP32 (Arduino + PubSubClient)

```cpp
#include <WiFi.h>
#include <PubSubClient.h>

const char* ssid = "SUA_REDE";
const char* wifiPassword = "SUA_SENHA";

const char* mqttServer = "SEU_SERVIDOR";  // IP ou hostname (ex: 192.168.1.100)
const int mqttPort = 1883;
const char* deviceName = "esp32-sala";      // username MQTT
const char* authToken = "TOKEN_DO_DEVICE";  // password MQTT (obtido ao registrar)

WiFiClient espClient;
PubSubClient mqtt(espClient);

void conectarMQTT() {
  while (!mqtt.connected()) {
    Serial.print("Conectando MQTT...");
    if (mqtt.connect(deviceName, deviceName, authToken)) {
      Serial.println(" conectado!");
    } else {
      Serial.printf(" falhou (rc=%d). Tentando em 5s...\n", mqtt.state());
      delay(5000);
    }
  }
}

void setup() {
  Serial.begin(115200);

  WiFi.begin(ssid, wifiPassword);
  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }
  Serial.println("\nWiFi conectado!");

  mqtt.setServer(mqttServer, mqttPort);
}

void enviarDados(float temperatura, float umidade) {
  if (!mqtt.connected()) conectarMQTT();
  mqtt.loop();

  String topic = "devices/" + String(deviceName) + "/data";
  String payload = "{\"device_id\":\"" + String(deviceName) + "\","
                   "\"temperatura\":" + String(temperatura, 1) + ","
                   "\"umidade\":" + String(umidade, 1) + "}";

  if (mqtt.publish(topic.c_str(), payload.c_str())) {
    Serial.println("Dados publicados via MQTT!");
  } else {
    Serial.println("Falha ao publicar.");
  }
}

void loop() {
  float temp = 23.5;  // substituir pela leitura real
  float hum = 60.0;
  enviarDados(temp, hum);
  delay(60000);
}
```

### Exemplo com Python (paho-mqtt)

```python
import paho.mqtt.client as mqtt
import json
import time

BROKER = "SEU_SERVIDOR"  # IP ou hostname
PORT = 1883
DEVICE_NAME = "sensor-python-01"
AUTH_TOKEN = "TOKEN_DO_DEVICE"  # obtido ao registrar no dashboard

client = mqtt.Client(client_id=DEVICE_NAME)
client.username_pw_set(DEVICE_NAME, AUTH_TOKEN)
client.connect(BROKER, PORT, 60)
client.loop_start()

topic = f"devices/{DEVICE_NAME}/data"

while True:
    payload = json.dumps({
        "device_id": DEVICE_NAME,
        "temperatura": 24.3,
        "umidade": 61.5
    })

    client.publish(topic, payload)
    print("Dados publicados!")
    time.sleep(60)
```

### Testando MQTT com mosquitto (CLI)

Se tiver o mosquitto-clients instalado, pode testar rapidamente:

```bash
mosquitto_pub -h localhost -p 1883 \
  -u "esp32-sala" -P "TOKEN_DO_DEVICE" \
  -t "devices/esp32-sala/data" \
  -m '{"device_id":"esp32-sala","temperatura":25.0,"umidade":60.0}'
```

---

## REST vs MQTT: Quando usar cada um?

| Criterio | REST (HTTP) | MQTT |
|----------|-------------|------|
| Simplicidade | Mais simples, basta fazer um POST | Precisa manter conexao ativa |
| Cloud (Render) | Funciona | Nao funciona (porta TCP bloqueada) |
| Consumo de energia | Maior (abre conexao a cada envio) | Menor (conexao persistente) |
| Latencia | Maior | Menor |
| Firewall | Porta 443 (HTTPS), quase sempre aberta | Porta 1883, pode ser bloqueada |
| Ideal para | Deploy na nuvem, envios esporadicos, prototipos | Rede local, envios frequentes, muitos dispositivos |
| Auth com API Key | Sim (sem cadastro previo) | Nao disponivel |
| Auth com Token | Sim | Sim (obrigatorio) |
| Auto-criacao | Sim (com API Key + `DEVICE_AUTO_CREATE=true`) | Nao, dispositivo deve ser registrado antes |

---

## Verificando os Dados

Apos enviar dados, voce pode verifica-los:

1. **Dashboard**: acesse o frontend e va na aba "Signal Values"
2. **API**: consulte os valores via REST:

```bash
# Listar dispositivos
curl -H "Authorization: Bearer <jwt-token>" https://go-pe.onrender.com/devices

# Listar sinais de um dispositivo
curl -H "Authorization: Bearer <jwt-token>" https://go-pe.onrender.com/devices/1/signals

# Listar valores de um sinal
curl -H "Authorization: Bearer <jwt-token>" https://go-pe.onrender.com/signals/1/values
```

---

## Solucao de Problemas

| Problema | Causa provavel | Solucao |
|----------|---------------|---------|
| `401 Unauthorized` (REST) | API key ou token incorreto | Verifique `X-API-Key` ou `Authorization` header |
| `400 Bad Request` | JSON invalido ou `device_id` ausente | Verifique o formato do payload |
| `Authorization header required` | Nenhum header de autenticacao enviado | Adicione `X-API-Key` ou `Authorization: Bearer` |
| MQTT nao conecta | Credenciais erradas ou broker desabilitado | Verifique `MQTT_BROKER_ENABLED=true` e as credenciais |
| MQTT nao conecta (Render) | Render nao expoe porta 1883 | Use REST no Render. MQTT so funciona local/VPS |
| MQTT publica mas dados nao aparecem | Topico incorreto | Use `devices/{nome}/data` exatamente |
| Dispositivo nao encontrado (MQTT) | Dispositivo nao registrado | Registre o dispositivo via dashboard ou API antes de conectar |
| Render demora para responder | Free tier entra em sleep | Primeira requisicao pode levar ~30s. E normal. |
