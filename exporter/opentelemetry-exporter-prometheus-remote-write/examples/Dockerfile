FROM python:3.7
WORKDIR /code

COPY . .
RUN apt-get update -y && apt-get install libsnappy-dev -y
RUN pip install -e .
RUN pip install -r ./examples/requirements.txt
CMD ["python", "./examples/sampleapp.py"]
