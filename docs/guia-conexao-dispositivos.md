# Guia de Conexão de Dispositivos

Como conectar um dispositivo IoT (ESP32, Arduino, Raspberry Pi, etc.) à API de armazenamento de dados.

Existem duas opções: **REST (HTTP)** e **MQTT**. Ambas aceitam o mesmo formato de payload JSON.

---

## Obtendo Credenciais

Antes de conectar um dispositivo, você precisa de credenciais de autenticação.

### Opção A: API Key Global (mais simples para REST)

A API Key é uma chave única compartilhada entre todos os dispositivos. Basta incluir no header `X-API-Key` das requisições REST.

```
API Key: klausf@bricio*030326
```

Com a API Key, dispositivos são criados automaticamente no primeiro envio (se `DEVICE_AUTO_CREATE=true`). Não precisa cadastrar o dispositivo antes.

### Opção B: Token Individual por Dispositivo (REST e MQTT)

Cada dispositivo recebe um token único ao ser registrado. Necessário para MQTT e também funciona como alternativa à API Key no REST.

**Pelo Dashboard (recomendado):**

1. Acesse o dashboard web e faça login
2. Vá na aba **Dispositivos**
3. Clique em **Adicionar Dispositivo**
4. Preencha nome, tipo e localização
5. Após criar, um dialog aparece com o **Auth Token** — **copie e salve imediatamente**, ele não será exibido novamente com destaque
6. Na tabela de dispositivos, a coluna **Auth Token** mostra os primeiros caracteres com um botão de copiar para obter o token completo

**Pela API:**

```bash
curl -X POST https://seu-servidor.com/auth/register-device \
  -H "Authorization: Bearer <token-usuario>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "esp32-sala",
    "description": "Sensor de temperatura da sala",
    "device_type": "sensor",
    "location": "Sala"
  }'
```

A resposta inclui o `auth_token` do dispositivo.

---

## Formato do Payload

```json
{
  "device_id": "meu-sensor-01",
  "field_1": 23.5,
  "field_2": 65,
  "field_3": true,
  "field_4": "status-ok"
}
```

- `device_id` (obrigatório): nome único do dispositivo
- `field_1` a `field_8` (opcionais): envie apenas os campos que tiver
  - **Número** → salvo como sinal analógico
  - **Booleano** → salvo como sinal digital
  - **String** → salvo nos metadados
- Dispositivos e sinais são criados automaticamente no primeiro envio (se usando API Key com `DEVICE_AUTO_CREATE=true`)

---

## Opção 1: REST (HTTP POST)

O endpoint é `POST /devices/data`.

### Autenticação

Escolha **uma** das opções:

| Método | Header | Quando usar |
|--------|--------|-------------|
| API Key | `X-API-Key: klausf@bricio*030326` | Mais simples, dispositivo criado automaticamente |
| Token do dispositivo | `Authorization: Bearer <token>` | Dispositivo já registrado, mais seguro por dispositivo |

### Exemplo com cURL

```bash
# Com API Key (mais simples — dispositivo criado automaticamente)
curl -X POST https://seu-servidor.com/devices/data \
  -H "Content-Type: application/json" \
  -H "X-API-Key: klausf@bricio*030326" \
  -d '{
    "device_id": "sensor-temperatura-01",
    "field_1": 24.3,
    "field_2": 61.5
  }'

# Com token do dispositivo (registrado previamente)
curl -X POST https://seu-servidor.com/devices/data \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer token-do-dispositivo" \
  -d '{
    "device_id": "sensor-temperatura-01",
    "field_1": 24.3,
    "field_2": 61.5
  }'
```

Resposta de sucesso: `201 Created` com `{"status": "ok"}`.

### Exemplo com ESP32 (Arduino)

```cpp
#include <WiFi.h>
#include <HTTPClient.h>

const char* ssid = "SUA_REDE";
const char* password = "SUA_SENHA";
const char* serverUrl = "https://seu-servidor.com/devices/data";
const char* apiKey = "klausf@bricio*030326";

void setup() {
  Serial.begin(115200);
  WiFi.begin(ssid, password);
  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }
  Serial.println("\nWiFi conectado!");
}

void enviarDados(float temperatura, float umidade) {
  if (WiFi.status() != WL_CONNECTED) return;

  HTTPClient http;
  http.begin(serverUrl);
  http.addHeader("Content-Type", "application/json");
  http.addHeader("X-API-Key", apiKey);

  String payload = "{\"device_id\":\"esp32-sala\","
                   "\"field_1\":" + String(temperatura, 1) + ","
                   "\"field_2\":" + String(umidade, 1) + "}";

  int httpCode = http.POST(payload);
  if (httpCode == 201) {
    Serial.println("Dados enviados com sucesso!");
  } else {
    Serial.printf("Erro HTTP: %d\n", httpCode);
  }
  http.end();
}

void loop() {
  float temp = 23.5;  // substituir pela leitura real do sensor
  float hum = 60.0;
  enviarDados(temp, hum);
  delay(60000); // envia a cada 60 segundos
}
```

### Exemplo com MicroPython (ESP32/ESP8266)

```python
import urequests
import ujson
import time

SERVER_URL = "https://seu-servidor.com/devices/data"
API_KEY = "klausf@bricio*030326"

def enviar_dados(temperatura, umidade):
    payload = ujson.dumps({
        "device_id": "esp32-sala",
        "field_1": temperatura,
        "field_2": umidade
    })

    headers = {
        "Content-Type": "application/json",
        "X-API-Key": API_KEY
    }

    try:
        response = urequests.post(SERVER_URL, data=payload, headers=headers)
        print("Status:", response.status_code)
        response.close()
    except Exception as e:
        print("Erro:", e)

while True:
    temp = 23.5  # substituir pela leitura real
    hum = 60.0
    enviar_dados(temp, hum)
    time.sleep(60)
```

---

## Opção 2: MQTT

O servidor possui um broker MQTT embutido. O dispositivo conecta via MQTT e publica mensagens no tópico `devices/{nome-do-dispositivo}/data`.

### Pré-requisitos

O broker precisa estar habilitado no servidor:

```env
MQTT_BROKER_ENABLED=true
MQTT_BROKER_PORT=1883
```

### Autenticação MQTT

- **Username**: nome do dispositivo (mesmo valor de `name` no banco)
- **Password**: `auth_token` do dispositivo

O dispositivo precisa estar cadastrado previamente. Registre pelo **dashboard** (aba Dispositivos → Adicionar Dispositivo) ou pela API conforme descrito na seção [Obtendo Credenciais](#obtendo-credenciais).

### Tópico

O dispositivo só pode publicar no seu próprio tópico:

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

const char* mqttServer = "seu-servidor.com";
const int mqttPort = 1883;
const char* deviceName = "esp32-sala";      // username MQTT
const char* authToken = "token-do-device";  // password MQTT (obtido ao registrar)

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
                   "\"field_1\":" + String(temperatura, 1) + ","
                   "\"field_2\":" + String(umidade, 1) + "}";

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

BROKER = "seu-servidor.com"
PORT = 1883
DEVICE_NAME = "sensor-python-01"
AUTH_TOKEN = "token-do-device"  # obtido ao registrar no dashboard

client = mqtt.Client(client_id=DEVICE_NAME)
client.username_pw_set(DEVICE_NAME, AUTH_TOKEN)
client.connect(BROKER, PORT, 60)
client.loop_start()

topic = f"devices/{DEVICE_NAME}/data"

while True:
    payload = json.dumps({
        "device_id": DEVICE_NAME,
        "field_1": 24.3,
        "field_2": 61.5
    })

    client.publish(topic, payload)
    print("Dados publicados!")
    time.sleep(60)
```

---

## REST vs MQTT: Quando usar cada um?

| Critério | REST (HTTP) | MQTT |
|----------|-------------|------|
| Simplicidade | Mais simples, basta fazer um POST | Precisa manter conexão ativa |
| Consumo de energia | Maior (abre conexão a cada envio) | Menor (conexão persistente) |
| Latência | Maior | Menor |
| Firewall | Porta 443 (HTTPS), quase sempre aberta | Porta 1883, pode ser bloqueada |
| Ideal para | Envios esporádicos, protótipos rápidos | Envios frequentes, muitos dispositivos |
| Auth com API Key | Sim (sem cadastro prévio) | Não disponível |
| Auth com Token | Sim | Sim (obrigatório) |
| Auto-criação | Sim (com API Key + `DEVICE_AUTO_CREATE=true`) | Não, dispositivo deve ser registrado antes |

---

## Verificando os Dados

Após enviar dados, você pode verificá-los:

1. **Dashboard**: acesse o frontend e vá na aba "Signal Values"
2. **API**: consulte os valores via REST:

```bash
# Listar dispositivos
curl -H "Authorization: Bearer <token>" https://seu-servidor.com/devices

# Listar sinais de um dispositivo
curl -H "Authorization: Bearer <token>" https://seu-servidor.com/devices/1/signals

# Listar valores de um sinal
curl -H "Authorization: Bearer <token>" https://seu-servidor.com/signals/1/values
```

---

## Solução de Problemas

| Problema | Causa provável | Solução |
|----------|---------------|---------|
| `401 Unauthorized` (REST) | API key ou token incorreto | Verifique `X-API-Key` ou `Authorization` header |
| `400 Bad Request` | JSON inválido ou `device_id` ausente | Verifique o formato do payload |
| MQTT não conecta | Credenciais erradas ou broker desabilitado | Verifique `MQTT_BROKER_ENABLED=true` e as credenciais |
| MQTT publica mas dados não aparecem | Tópico incorreto | Use `devices/{nome}/data` exatamente |
| Dispositivo não encontrado (MQTT) | Dispositivo não registrado | Registre o dispositivo via dashboard ou API antes de conectar |
