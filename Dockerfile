FROM python:3.11-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    STHIRA_PROFILE=DEMO

WORKDIR /app
COPY pyproject.toml ./
COPY src ./src
RUN pip install --no-cache-dir .

# Build with --build-arg INCLUDE_LOCAL_VOICE_RUNTIME=true only in a reviewed
# model-serving environment. Model weights are always mounted read-only.
ARG INCLUDE_LOCAL_VOICE_RUNTIME=false
COPY requirements-voice.txt ./
RUN if [ "$INCLUDE_LOCAL_VOICE_RUNTIME" = "true" ]; then pip install --no-cache-dir -r requirements-voice.txt; fi

EXPOSE 8000
CMD ["uvicorn", "sthira.api.app:app", "--host", "0.0.0.0", "--port", "8000"]
