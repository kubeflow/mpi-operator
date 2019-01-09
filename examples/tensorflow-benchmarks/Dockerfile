FROM uber/horovod:0.15.2-tf1.12.0-torch1.0.0-py2.7

# Temporary fix until Horovod pushes out a new release.
# See https://github.com/uber/horovod/pull/700
RUN sed -i '/^NCCL_SOCKET_IFNAME.*/d' /etc/nccl.conf

RUN mkdir /tensorflow
WORKDIR "/tensorflow"
RUN git clone -b cnn_tf_v1.12_compatible https://github.com/tensorflow/benchmarks
WORKDIR "/tensorflow/benchmarks"

CMD mpirun \
  python scripts/tf_cnn_benchmarks/tf_cnn_benchmarks.py \
    --model resnet101 \
    --batch_size 64 \
    --variable_update horovod
