FROM horovod/horovod:0.16.4-tf1.14.0-torch1.1.0-mxnet1.4.1-py3.6

RUN mkdir /tensorflow
WORKDIR "/tensorflow"
RUN git clone https://github.com/tensorflow/benchmarks
WORKDIR "/tensorflow/benchmarks"

CMD ["/bin/bash"]
