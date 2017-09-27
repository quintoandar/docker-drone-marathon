FROM python:2-alpine
ADD . /usr/src/app
RUN pip install requests
ENTRYPOINT ["python", "/usr/src/app/entrypoint.py"]
