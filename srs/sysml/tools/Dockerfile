# Окружение для рендера: SysML v2 Pilot Implementation 0.62.0 (релиз 2026-08) + Graphviz + Java 21
FROM condaforge/miniforge3:24.11.3-0
RUN conda install -y -c conda-forge "jupyter-sysml-kernel=0.62.0" "graphviz=2.*" jupyter nbconvert nbclient cairosvg \
 && conda clean -afy
COPY --from=eclipse-temurin:21.0.4_7-jre /opt/java/openjdk /opt/jdk21
RUN ln -sf /opt/jdk21/bin/java /opt/conda/bin/java \
 && apt-get update && apt-get install -y --no-install-recommends fontconfig fonts-dejavu-core \
 && rm -rf /var/lib/apt/lists/*
ENV JAVA_HOME=/opt/jdk21 PATH=/opt/jdk21/bin:$PATH JAVA_TOOL_OPTIONS=-Djava.awt.headless=true
WORKDIR /work
