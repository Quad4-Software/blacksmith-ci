<!-- markdownlint-disable MD041 -->

# Ansible

If you are familiar with Ansible, you can use the `crowci` role within the [`devxy.cicd`](https://galaxy.ansible.com/ui/repo/published/devxy/cicd/) collection to install Crow.

It uses a container-based approach to install Crow (as opposed to a traditional host install which is what Ansible does most often) as this allows for multiple agents on the same host.

## Installation

```sh
ansible-galaxy collection install devxy.cicd
```

or via `requirements.yml`:

```yaml
collections:
  - name: devxy.cicd
    version: <version>
```

## Usage

👉️ [All variable definitions](https://codefloe.com/devxy/ansible-cicd/src/branch/main/roles/crowci#default-variables)

```yaml
- name: Crow CI
  vars:
    - [...]
  roles:
    - role: devxy.cicd.crowci
```
