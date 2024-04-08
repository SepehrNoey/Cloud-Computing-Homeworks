FROM python:3.9.19-alpine

WORKDIR /app

COPY cc_hw2_hello.py /app/
COPY movies.json /app/

RUN pip install numpy
CMD [ "python", "cc_hw2_hello.py" ]