# Base image for MPIOperator with DeepSpeed and CUDA setup
FROM mpioperator/deepspeedbase

# Select WORKDIR for cifar tutorial
WORKDIR /deepspeed/DeepSpeedExamples/training/cifar

# Install dependencies
RUN pip3 install pillow \
    matplotlib

# Run the script for running DeepSpeed applied model
CMD [ "sh", "run_ds.sh" ]
