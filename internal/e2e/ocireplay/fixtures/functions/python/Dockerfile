FROM fnproject/python:3.11-dev@sha256:86e385f8fa59bd5a38edf8486778cf457379d6352713ccbb2eb3221e2d39cc93 AS build

WORKDIR /function
COPY requirements.txt .
RUN pip3 install --target /python --no-cache-dir -r requirements.txt
COPY func.py .

FROM fnproject/python:3.11@sha256:28857e94288268d4dcae7a06ffa2c8c93b180f9bbfa642a5c165f965cbec2775
WORKDIR /function
COPY --from=build /function /function
COPY --from=build /python /python
ENV PYTHONPATH=/function:/python
ENTRYPOINT ["/python/bin/fdk", "/function/func.py", "handler"]
