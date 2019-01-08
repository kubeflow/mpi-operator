FROM uber/horovod:0.15.2-tf1.12.0-torch1.0.0-py2.7

RUN mkdir /tensorflow
WORKDIR "/tensorflow"
RUN git clone -b cnn_tf_v1.12_compatible https://github.com/tensorflow/benchmarks
WORKDIR "/tensorflow/benchmarks"

CMD mpirun \
  python scripts/tf_cnn_benchmarks/tf_cnn_benchmarks.py \
    --model resnet101 \
    --batch_size 64 \
    --variable_update horovod
