FROM uber/horovod:0.13.3-tf1.8.0-torch0.4.0-py2.7

RUN mkdir /tensorflow
WORKDIR "/tensorflow"
RUN git clone https://github.com/tensorflow/benchmarks
WORKDIR "/tensorflow/benchmarks"
# Go back to a commit that's compatible with tf1.8.0:
RUN git reset --hard 3b90c14

CMD mpirun \
  python scripts/tf_cnn_benchmarks/tf_cnn_benchmarks.py \
    --model resnet101 \
    --batch_size 64 \
    --variable_update horovod
