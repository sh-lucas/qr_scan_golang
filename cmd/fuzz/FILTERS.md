# Fuzzing Filters Overview

Este fuzzer utiliza um conjunto de filtros de processamento de imagem para tentar extrair QR Codes de imagens degradadas. Cada filtro funciona como um iterador, gerando variações de parâmetros baseadas no flag `-step`.

## Glossário de Filtros e Intervalos

### 1. Bilateral Filter
Filtro de suavização que preserva bordas, ideal para remover ruído sem "borrar" o QR Code.
- **D (Diameter)**: `3` a `7`
- **Sigma (Color/Space)**: `10.0` a `50.0`
- **Impacto do Step**: Incrementa `D` em `2 * step` e `Sigma` em `15.0 * step`.

### 2. Gamma Correction
Ajusta a luminância da imagem via Power-law. Útil para fotos subexpostas ou superexpostas.
- **Gamma**: `0.6` a `1.8`
- **Impacto do Step**: Incrementa em `0.4 * step`.

### 3. CLAHE (Contrast Limited Adaptive Histogram Equalization)
Equalização de histograma adaptativa que aumenta o contraste local.
- **Clip Limit**: `1.0` a `3.5`
- **Tile Size**: `4x4` a `12x12`
- **Impacto do Step**: Incrementa `Clip` em `0.5 * step` e `Tile` em `4 * step`.

### 4. Resize
Redimensiona a imagem (Upscaling e Downscaling).
- **Scale**: `0.5`, `0.75`, `1.25`, `1.5`, `1.75`, `2.0` (pula `1.0`)
- **Impacto do Step**: Incrementa em `0.25 * step`.

### 5. Sharpen (Unsharp Mask)
Aumenta a nitidez das bordas do QR Code.
- **Alpha (Intensidade)**: `1.0` a `2.0`
- **Impacto do Step**: Incrementa em `0.5 * step`.

### 6. Adaptive Threshold
Binarização local baseada na vizinhança do pixel.
- **Block Size**: `3` a `21`
- **C (Constante)**: `2.0` a `10.0`
- **Métodos**: `Mean` e `Gaussian`
- **Impacto do Step**: Incrementa `Block` em `2 * step` e `C` em `2.0 * step`.

### 7. Edge Contrast (Desabilitado)
Detecta bordas (Canny) e as sobrepõe na imagem original para forçar o contraste.
- **Canny Low**: `50` a `150`
- **Canny High**: `100` a `300`
- **Edge Alpha**: `0.1` a `0.3`
- **Impacto do Step**: Incrementa limites em `25 * step` e `Alpha` em `0.2 * step`.

### 8. Black Hat Morphological (Desabilitado)
Destaca detalhes escuros em fundos claros. Excelente para QR Codes pretos em papel branco com sombras.
- **Kernel Size**: `3` a `21`
- **Impacto do Step**: Incrementa em `2 * step`.

### 9. Dilation (Dilatação)
Expande as áreas claras (pixels brancos). Ajuda se o QR está com falhas de impressão ou "fino" demais.
- **Kernel Size**: `3` a `5`
- **Impacto do Step**: Incrementa em `2 * step`.

### 10. Morphological Closing
Combo de Dilatação + Erosão. Fecha pequenos buracos e conecta blocos separados do QR Code.
- **Kernel Size**: `3` a `9`
- **Impacto do Step**: Incrementa em `2 * step`.

---

## Como as Permutações são Geradas
O fuzzer utiliza uma abordagem **BFS (Breadth-First Search)**:
1. Testa todas as combinações de **1 filtro** primeiro.
2. Se não encontrar sucesso, testa todas as combinações de **2 filtros** em cadeia.
3. Se o flag `-max-length` permitir, segue para cadeias de **3 filtros**.

**Nota**: Assim que uma imagem é resolvida em uma camada (ex: Camada 1), o fuzzer termina o processamento dessa imagem após completar toda a varredura daquela mesma camada, garantindo que pegamos múltiplos "hits" simples antes de tentar o "brute-force" pesado das camadas profundas.
