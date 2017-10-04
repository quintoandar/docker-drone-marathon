FROM python:2-alpine
RUN pip install requests
ADD . /usr/src/app
ENTRYPOINT ["python", "/usr/src/app/entrypoint.py"]
