# QR Scan Golang

Este projeto oferece uma ferramenta de linha de comando (CLI) super rápida e com altíssima taxa de sucesso para extrair links de QR Codes a partir de imagens complexas (físicamente amassadas, com ruído, iluminação distorcida, resoluções variadas ou em ângulos inclinados).

## 🚀 Como funciona?

Enquanto bibliotecas Go puras (como `goqr` ou `gozxing`) dependem de algoritmos clássicos de leitura que falham na vida real ao lidar com imagens sujas de câmeras de celular, a nossa solução integra a poderosa engine **WeChat QR**, criada pela Tencent, cujo detector inicial e decodificador utilizam **Machine Learning e Redes Neurais Convolucionais** rodando nativamente sobre o `OpenCV` através da biblioteca `gocv`.

Além disso, nosso pipeline em código Go implementa **processamento avançado da matriz da imagem** através do próprio OpenCV, realizando múltiplas passagens automáticas sob falhas, incluindo:
1. Leitura direta através dos modelos Caffe
2. *Gaussian Blur* (ideal para reduzir ruídos de ISO de celulares)
3. *CLAHE* (*Contrast Limited Adaptive Histogram Equalization* para equalização adaptativa da tinta do papel)
4. *Bicubic Scaler 1.5x* 

## ⚙️ Dependências Técnicas da Aplicação

- **Framework**: Linguagem Go 1.23.2
- **Driver CV**: `gocv.io/x/gocv` (Binding de Go para o OpenCV nativo da máquina em C++)
- **Core OpenCV**: OpenCV `v4.10.x` estático + Módulos do `opencv-contrib` (que inclui a feature `wechat_qrcode`).
- **Deep Learning Models**:
    - `detect.prototxt` & `detect.caffemodel`: Usados para descobrir fisicamente a posição em X e Y dos marcadores do QR Code.
    - `sr.prototxt` & `sr.caffemodel`: Super-Resolução (amplia imagens de baixa qualidade para o solver conseguir traduzir com inteligência artificial).

## 🏗️ Ambiente & Docker

Compilar OpenCV localmente consumiria tempo absurdo. O jeito idiomático (e incrivelmente mais rápido e seguro) que implementamos para o projeto rodar na sua pipeline ou máquina local é utilizar **Docker**.

O `Dockerfile` usa **Multi-Stage Build**:
1. Usa uma imagem Dockerfile já consolidada com todo o ambiente CGO & OpenCV 4.10 estático rodando no kernel Alpine/Debian `ghcr.io/hybridgroup/opencv:4.10.0`.
2. Compila a sub-rotina do nosso script (`main.go`). Essa fase constrói muito rápido (cerca de ~5 s).
3. Transpõe apenas o binário executável gerado, otimizando drasticamente o peso final para ser executado.

### Executando em Container (Recomendado)

O `Makefile` possui as tags automatizadas para as execuções:
```bash
# 1. Baixar as redes neurais Caffe (~1.5mb) e inicializar a pasta `models/`
make models

# 2. Realizar o Multi-Stage build gerando a imagem Docker final (`qr-scanner`)
make docker-build

# 3. Mapear o volume local e executar o scanner recursivamente numa pasta
make docker-run
```

Se desejar executar de forma livre via Go sem Makefile, usando Docker Compose ou standalone localmente ignorando os containers (necessita CGO habilitado e libopencv instaladas via OS):
```bash
go run main.go ./minha-pasta/de-imagens/
```

## 🧩 Interface de Classes Internas (Go)

A essência do código foi modularizada no pacote `scanner`, o qual expõe o wrapper da struct `WeChatQRScanner`. Toda a abstração suja de lidar com CGO Arrays memory clean-up (através do `.Close()` dos Mats gerados pelo GoCV) ficam enclausuradas dentro deste pacote seguro.  

```go
package scanner

// NewWeChatQRScanner:
// Inicializador do serviço de scanning. Requer o acesso aos 4 arquivos Caffe da Tencent baixados via `make models` e presentes no diretório alvo.
// Devolve uma instancia do driver com a struct inicializada.
func NewWeChatQRScanner(modelsDir string) (*WeChatQRScanner, error)

// (s *WeChatQRScanner) Scan(imagePath string):
// Interface principal sendo utilizada pelo Entrypoint do projeto (CLI). 
// Recebe apenas o Absolute Path do arquivo alvo. 
// A função devolve um Array de Strings limpa []string com TODAS as URL's descriptografadas e formatadas na imagem recebida.
func (s *WeChatQRScanner) Scan(imagePath string) ([]string, error)
```
