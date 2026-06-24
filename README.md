# 🚦 Traefik Blue/Green Plugin

## Visão Geral

O Traefik Blue/Green Plugin é um middleware desenvolvido em Go para suportar estratégias de deploy Blue/Green baseadas em **tenant** e **aplicação**.

O plugin atua como uma camada de decisão entre o recebimento da requisição e o roteamento final realizado pelo Traefik, permitindo direcionar diferentes tenants para diferentes versões da mesma aplicação.

A decisão de roteamento é baseada em informações armazenadas no Redis e mantidas localmente em cache para reduzir latência e minimizar consultas externas.

---

# Arquitetura

## Componentes

- Cliente
- Traefik
- Plugin Blue/Green
- Cache Local
- Redis
- HTTPRoute
- Serviços Blue e Green

---

# Fluxo de Requisição

1. O cliente envia uma requisição para uma aplicação.

Exemplo:

```http
GET /nginx?tenant=123
```

2. O Traefik encaminha a requisição para o middleware Blue/Green.

3. O plugin extrai:
   - `AppSlug` a partir do path da requisição.
   - `Tenant` a partir do QueryParam `tenant`.

Exemplo:

```text
/nginx?tenant=123

AppSlug = nginx
Tenant  = 123
```

4. O plugin monta a chave de consulta:

```text
123:nginx
```

5. O plugin consulta o cache local.

6. Caso exista uma entrada válida:
   - Recupera o slot.
   - Continua o processamento.

7. Caso não exista:
   - Consulta o Redis utilizando `HGETALL`.
   - Recupera o valor do campo `slot`.
   - Armazena o resultado no cache local.

8. O plugin adiciona o header:

```http
X-Slot: 1
```

ou

```http
X-Slot: 2
```

9. O plugin realiza um proxy da requisição para o Traefik.

10. O Traefik processa novamente a requisição.

11. O HTTPRoute identifica o valor do header `X-Slot`.

12. A requisição é encaminhada para o backend correspondente.

---

# Fluxo Arquitetural

                                ┌─────────────────────┐
                                │      CLIENTE        │
                                └──────────┬──────────┘
                                           │
                                           │ GET /nginx/?tenant=123
                                           │
                                           ▼
                                ┌─────────────────────┐
                                │      TRAEFIK        │
                                │  EntryPoint / GW    │
                                └──────────┬──────────┘
                                           │
                                           ▼
                     ┌─────────────────────────────────────────────────┐
                     │            BLUE/GREEN PLUGIN                    │
                     └─────────────────────┬───────────────────────────┘
                                           │
                                           ▼
                            ┌──────────────────────────┐
                            │ Extrai informações       │
                            │                          │
                            │ AppSlug = nginx          │
                            │ Tenant  = 123            │
                            └──────────┬───────────────┘
                                       │
                                       ▼
                            ┌──────────────────────────┐
                            │ Chave de Consulta        │
                            │                          │
                            │ 123:nginx                │
                            └──────────┬───────────────┘
                                       │
                                       ▼
                       ┌────────────────────────────────┐
                       │        CACHE LOCAL             │
                       │                                │
                       │ map + sync.RWMutex             │
                       └──────────────┬─────────────────┘
                                      │
                   ┌──────────────────┴──────────────────┐
                   │                                     │
                   │ HIT                                 │ MISS
                   │                                     │
                   ▼                                     ▼
         ┌───────────────────┐           ┌──────────────────────────┐
         │ Recupera Slot     │           │ Cliente Redis RESP       │
         │                   │           │                          │
         │ slot=1            │           │ HGETALL 123:nginx        │
         └─────────┬─────────┘           └────────────┬─────────────┘
                   │                                  │
                   │                                  ▼
                   │                    ┌───────────────────────────┐
                   │                    │          REDIS            │
                   │                    │                           │
                   │                    │ Hash: 123:nginx           │
                   │                    │                           │
                   │                    │ tenant = 123              │
                   │                    │ app    = ngin. x          │
                   │                    │ slot   = 1                │
                   │                    └─────────────┬─────────────┘
                   │                                  │
                   │                                  ▼
                   │                    ┌──────────────────────────┐
                   │                    │ Desserializa RESP        │
                   │                    │ Atualiza Cache Local     │
                   │                    └────────────┬─────────────┘
                   │                                 │
                   └──────────────┬──────────────────┘
                                  │
                                  ▼
                     ┌──────────────────────────────┐
                     │ Recupera valor do Slot       │
                     │                              │
                     │ slot = 1                     │
                     └──────────────┬───────────────┘
                                    │
                                    ▼
                     ┌──────────────────────────────┐
                     │ Adiciona Header              │
                     │                              │
                     │ X-Slot: 1                    │
                     └──────────────┬───────────────┘
                                    │
                                    ▼
                     ┌──────────────────────────────┐
                     │ Proxy para o Traefik         │
                     │                              │
                     │ TraefikProxyURL             │
                     └──────────────┬───────────────┘
                                    │
                                    ▼
                     ┌──────────────────────────────┐
                     │      TRAEFIK (NOVAMENTE)     │
                     └──────────────┬───────────────┘
                                    │
                                    ▼
                     ┌──────────────────────────────┐
                     │        HTTPRoute             │
                     │                              │
                     │ Header X-Slot = 1 ?          │
                     └──────────────┬───────────────┘
                                    │
                    ┌───────────────┴────────────────┐
                    │                                │
                    ▼                                ▼
          ┌──────────────────┐             ┌──────────────────┐
          │     nginx-01     │             │     nginx-02     │
          │      BLUE        │             │      GREEN       │
          │    X-Slot=1      │             │    X-Slot=2      │
          └────────┬─────────┘             └────────┬─────────┘
                   │                                │
                   └──────────────┬─────────────────┘
                                  │
                                  ▼
                     ┌──────────────────────────────┐
                     │      RESPONSE CLIENTE        │
                     └──────────────────────────────┘

---

# Estrutura de Dados

## Redis

O Redis é utilizado como fonte de verdade para determinar o slot associado a cada tenant e aplicação.

### Chave

```text
tenantID:appSlug
```

Exemplo:

```text
123:nginx
```

### Estrutura

Cada chave é armazenada como um Hash Redis.

Exemplo:

```redis
HSET 123:nginx \
tenant 123 \
app nginx \
slot 1
```

### Consulta

```redis
HGETALL 123:nginx
```

### Retorno

```text
tenant = 123
app    = nginx
slot   = 1
```

O plugin utiliza exclusivamente o valor do campo `slot` para determinar o roteamento.

---

# Comunicação com Redis

O plugin implementa um cliente Redis próprio utilizando o protocolo RESP (Redis Serialization Protocol).

Não são utilizadas bibliotecas externas para comunicação com Redis.

A implementação contempla:

- Serialização de comandos RESP
- Desserialização das respostas RESP
- Comunicação TCP direta com Redis
- Conversão das respostas para estruturas internas do plugin

## Exemplo de Serialização

Comando:

```redis
HGETALL 123:nginx
```

Formato RESP:

```text
*2
$7
HGETALL
$9
123:nginx
```

## Exemplo de Resposta

```text
*6
$6
tenant
$3
123
$3
app
$5
nginx
$4
slot
$1
1
```

Após desserialização:

```go
type TenantSlot struct {
	TenantID string
	AppName  string
	Slot     string
}
```

```go
...
&models.TenantSlot{
    TenantID: "123",
    AppName:  "nginx",
    Slot:     "1",
}
...
```

---

# Cache Local

O plugin mantém um cache em memória para reduzir a quantidade de consultas ao Redis.

## Estrutura

```go
type cacheEntry struct {
	tenant    *models.TenantSlot
	expiresAt time.Time
}
```

```go
type LocalCache struct {
	cache map[string]*cacheEntry
	mu    sync.RWMutex
}
```

## Controle de Concorrência

O cache utiliza `sync.RWMutex`.

### Leitura

```go
mu.RLock()
```

Permite múltiplas leituras simultâneas.

### Escrita

```go
mu.Lock()
```

Garante exclusão mútua durante modificações.

## Benefícios

- Menor latência
- Redução de consultas Redis
- Melhor throughput
- Segurança para acesso concorrente

---

# Configuração

O middleware pode ser configurado através do recurso Traefik Middleware.

```yaml
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: bluegreen
spec:
  plugin:
    bluegreen:
      CacheTTL: ""
      RedisAddress: ""
      RedisDataBase: "0"
      RedisPort: "6379"
      TraefikProxyURL: ""
```

## Parâmetros

| Campo | Descrição |
|---------|---------|
| CacheTTL | Tempo de vida das entradas do cache local (segundos) |
| RedisAddress | Endereço do Redis |
| RedisPort | Porta do Redis |
| RedisDataBase | Banco Redis utilizado |
| TraefikProxyURL | URL utilizada para reenviar a requisição ao Traefik |

---

# Valores Padrão

Os seguintes valores são automaticamente aplicados caso não sejam informados:

| Campo | Valor Padrão |
|---------|---------|
| CacheTTL | 60 segundos |
| RedisPort | 6379 |
| TraefikProxyURL | https://traefik.traefik-controller.svc.cluster.local:443 |

---

# Roteamento

O plugin define o header `X-Slot`, que é utilizado pelo Traefik para determinar o backend.

## Slot 1 (Blue)

```yaml
- backendRefs:
    - kind: Service
      name: nginx-01
      port: 80
  matches:
    - path:
        type: PathPrefix
        value: /nginx
      headers:
        - name: X-Slot
          type: Exact
          value: "1"
```

## Slot 2 (Green)

```yaml
- backendRefs:
    - kind: Service
      name: nginx-02
      port: 80
  matches:
    - path:
        type: PathPrefix
        value: /nginx
      headers:
        - name: X-Slot
          type: Exact
          value: "2"
```

---

# Algoritmo

```text
1. Recebe requisição
2. Extrai AppSlug do path
3. Extrai tenant da query string
4. Monta chave tenant:app
5. Consulta cache local
6. Caso não exista:
      executa HGETALL no Redis
7. Recupera o campo slot
8. Atualiza cache local
9. Adiciona header X-Slot
10. Realiza proxy para o Traefik
11. HTTPRoute seleciona backend
12. Requisição é entregue ao serviço
```

---

# Considerações Técnicas

## Redis como Source of Truth

Toda decisão de roteamento é baseada nas informações armazenadas no Redis.

O cache local é utilizado apenas como mecanismo de otimização.

## Reentrada no Traefik

Após determinar o slot e adicionar o header `X-Slot`, o plugin realiza um proxy para o Traefik.

Isso permite manter toda a lógica de roteamento centralizada em recursos HTTPRoute, sem que o plugin precise conhecer detalhes dos serviços de destino.

## Concorrência

O cache local é protegido por `sync.RWMutex`, permitindo:

- Leituras simultâneas
- Escritas exclusivas
- Segurança para múltiplas requisições concorrentes

---

# Resumo

O plugin atua como uma camada de decisão para ambientes Blue/Green baseada em tenant e aplicação.

```text
Request
   │
   ▼
Plugin
   │
   ├─ Cache Local
   │
   ├─ Redis (HGETALL)
   │
   ▼
Header X-Slot
   │
   ▼
Proxy para Traefik
   │
   ▼
HTTPRoute
   │
   ├─ Slot 1 → nginx-01
   └─ Slot 2 → nginx-02
```

Dessa forma, a lógica de seleção do ambiente fica centralizada no plugin, enquanto o Traefik continua responsável exclusivamente pelo roteamento final.