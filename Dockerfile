FROM python:2-alpine

ADD . .

RUN pip install requests

ENTRYPOINT ["scripts/entrypoint.py"]