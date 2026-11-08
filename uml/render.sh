#!/usr/bin/env bash
# Рендер всех *.puml в images/*.png. Нужен Docker и plantuml.jar (PLANTUML_JAR).
# Graphviz берётся из образа sysml-render (srs/sysml/tools/Dockerfile).
set -euo pipefail
cd "$(dirname "$0")"
JAR="${PLANTUML_JAR:-$HOME/.vscode/extensions/jebbs.plantuml-2.18.1/plantuml.jar}"
docker run --rm -u "$(id -u):$(id -g)" -v "$PWD":/work -v "$JAR":/opt/plantuml.jar:ro \
  -w /work sysml-render:0.62 java -Dfile.encoding=UTF-8 -jar /opt/plantuml.jar \
  -charset UTF-8 -tpng -Sdpi=150 -o images $(ls *.puml)
