# Official MPI Operator Base image
FROM mpioperator/base

# mpi-operator mounts the .ssh folder from a Secret. For that to work, we need to disable UserKnownHostsFile to avoid write permissions.
# Disable StrictModes avoids directory and files read permission checks and update system packages & install dependencies
RUN apt-get update && apt-get install -y \
    git \
    wget \
    build-essential \
    cmake \
    libopenmpi-dev \
    openssh-server \
    python3 \
    python3-pip \
    && rm -rf /var/lib/apt/lists/* \
    && echo "    UserKnownHostsFile /dev/null" >> /etc/ssh/ssh_config \
    && sed -i 's/#\(StrictModes \).*/\1no/g' /etc/ssh/sshd_config

# Install DeepSpeed library and Torch with cu11.8 wheels
RUN pip3 install deepspeed
RUN pip3 install torch torchvision torchaudio --index-url https://download.pytorch.org/whl/cu118

# Create folder for deepspeed workspace
RUN mkdir /deepspeed

# Workspace for DeepSpeed examples
WORKDIR "/deepspeed"

# Clone the DeepSpeedExamples from repository
RUN git clone https://github.com/microsoft/DeepSpeedExamples/

# Set the working directory to DeepSpeedExamples for models
WORKDIR "/deepspeed/DeepSpeedExamples/"

# Set the default command to bash
CMD ["/bin/bash"]
