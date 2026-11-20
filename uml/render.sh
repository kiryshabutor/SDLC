#!/usr/bin/env bash
# Рендер всех *.puml в images/*.png. Нужен Docker и plantuml.jar (PLANTUML_JAR).
# Graphviz берётся из образа sysml-render (srs/sysml/tools/Dockerfile).
set -euo pipefail
cd "$(dirname "$0")"
JAR="${PLANTUML_JAR:-$HOME/.vscode/extensions/jebbs.plantuml-2.18.1/plantuml.jar}"
docker run --rm -u "$(id -u):$(id -g)" -v "$PWD":/work -v "$JAR":/opt/plantuml.jar:ro \
  -w /work sysml-render:0.62 java -Dfile.encoding=UTF-8 -jar /opt/plantuml.jar \
  -charset UTF-8 -tpng -Sdpi=150 -o images $(ls *.puml)

# Высокие узкие диаграммы дополняются белыми полями по бокам до отношения
# высоты к ширине 1,4: в отчёте все рисунки шириной 12 см, и такой рисунок
# не выше 17 см (ADR-0010, правило 9).
python3 - <<'PY'
import glob
from PIL import Image
for f in glob.glob('images/*.png'):
    im = Image.open(f)
    w, h = im.size
    if h / w > 1.4:
        nw = round(h / 1.4)
        out = Image.new(im.mode, (nw, h), 'white')
        out.paste(im, ((nw - w) // 2, 0))
        out.save(f)
PY
