# Docker

There are two ways to obtain a AMCDocker image:

1. [GitHub](#github)
2. [Building it from source](#building-the-docker-image)

Once you have obtained the Docker image, proceed to [Using the Docker
image](#using-the-docker-image).

> **Note**
>
> AMCrequires Docker Engine version 20.10.10 or higher due to [missing support](https://docs.docker.com/engine/release-notes/20.10/#201010) for the `clone3` syscall in previous versions.
## GitHub

amc docker images for both x86_64 and ARM64 machines are published with every release of amc on GitHub Container Registry.

You can obtain the latest image with:

```bash
docker pull amazechain/amc
```

Or a specific version (e.g. v0.1.0) with:

```bash
docker pull amazechain/amc:0.1.0
```

You can test the image with:

```bash
docker run --rm amazechain/amc:0.1.0-amd64
```

If you see the latest amc release version, then you've successfully installed amc via Docker.

## Building the Docker image

To build the image from source, navigate to the root of the repository and run:

```bash
make images
```

The build will likely take several minutes. Once it's built, test it with:

```bash
docker run amazechain/amc:local --version
```

## Using the Docker image

There are two ways to use the Docker image:
1. [Using Docker](#using-plain-docker)
2. [Using Docker Compose](#using-docker-compose)

### Using Plain Docker

To run amc with Docker, execute:

```
docker run -p 6060:6060 -p 61016: 61016 -p 61015: 61015/udp -v amcdata:/home/amc/data amazechain/amc:local --metrics --metrics.addr '0.0.0.0' 
```

The above command will create a container named amc and a named volume called amcdata for data persistence. It will also expose port 61016 TCP and 61015 UDP for peering with other nodes and port 6060 for metrics.

It will use the local image amc:local. If you want to use the DockerHub Container Registry remote image, use amazechain/amc with your preferred tag.

### Using Docker Compose

To run amc with Docker Compose, execute the following commands from a shell inside the root directory of this repository:

```bash
docker-compose -f docker-compose.yml up -d 
# or make up
```

The default `docker-compose.yml` file will create three containers:

- amc
- Prometheus
- Grafana


Grafana will be exposed on `localhost:3000` and accessible via default credentials (username and password is `admin`):

## Interacting with amc inside Docker

To interact with amc, you must first open a shell inside the amc container by running:

```bash
docker exec -it amc sh
```

**If amc is running with Docker Compose, replace amc with amc-amc-1 in the above command.**

Inside the amc container, refer to the [CLI docs](../cli/cli.md) documentation to interact with amc.