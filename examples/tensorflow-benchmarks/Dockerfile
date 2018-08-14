FROM uber/horovod:0.13.11-tf1.10.0-torch0.4.0-py2.7

RUN mkdir /tensorflow
WORKDIR "/tensorflow"
RUN git clone https://github.com/tensorflow/benchmarks
WORKDIR "/tensorflow/benchmarks"

CMD mpirun \
  python scripts/tf_cnn_benchmarks/tf_cnn_benchmarks.py \
    --model resnet101 \
    --batch_size 64 \
    --variable_update horovod
