# Official PyTorch image with CUDA support
FROM pytorch/pytorch:1.9.0-cuda11.1-cudnn8-runtime

# mpi-operator mounts the .ssh folder from a Secret. For that to work, we need to disable UserKnownHostsFile to avoid write permissions.
# Disable StrictModes avoids directory and files read permission checks and update system packages & install dependencies
RUN apt-get update && apt-get install -y \
    git \
    wget \
    build-essential \
    cmake \
    libopenmpi-dev \
    openssh-server \
    && rm -rf /var/lib/apt/lists/* \
    && echo "    UserKnownHostsFile /dev/null" >> /etc/ssh/ssh_config \
    && sed -i 's/#\(StrictModes \).*/\1no/g' /etc/ssh/sshd_config

# Install DeepSpeed library
RUN pip install deepspeed
RUN mkdir /deepspeed

# Workspace for DeepSpeed examples
WORKDIR "/deepspeed"

# Clone the DeepSpeedExamples from repository
RUN git clone https://github.com/microsoft/DeepSpeedExamples/

# Set the working directory to DeepSpeedExamples/training for models
WORKDIR "/deepspeed/DeepSpeedExamples/training"

# Set the default command to bash
CMD ["/bin/bash"]