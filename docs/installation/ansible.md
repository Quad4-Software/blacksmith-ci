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

## Encryption

Crow supports encrypting secrets, registry passwords, OAuth tokens, and forge client secrets at rest using [Google Tink](https://developers.google.com/tink).

### Generating a Tink keyset

First, install the `tinkey` CLI tool and generate a keyset:

```bash
# Install tinkey (see https://developers.google.com/tink/install-tinkey)
# Then generate a keyset:
tinkey create-keyset --key-template AES256_GCM --out-format json --out tink-keyset.json
```

/// warning
Keep your keyset file secure. If you lose it, you will not be able to decrypt existing secrets.
///

### Enabling encryption

Place your keyset file on the Ansible controller (e.g., in `files/tink-keyset.json`), then set the following variables:

```yaml
crow_encryption_enabled: true
crow_encryption_tink_keyset_file: "{{ playbook_dir }}/files/tink-keyset.json"
```

The role will:

1. Copy the keyset file to the target host at `crow_encryption_keyset_dest` (default: `/etc/crow/encryption/tink-keyset.json`)
2. Mount it into the server container at `crow_encryption_container_path`
3. Set the `CROW_ENCRYPTION_TINK_KEYSET_FILE` environment variable

### Custom paths

You can customize the paths if needed:

```yaml
crow_encryption_enabled: true
crow_encryption_tink_keyset_file: /path/to/local/keyset.json
crow_encryption_keyset_dest: /etc/crow/encryption/tink-keyset.json
crow_encryption_container_path: /etc/crow/encryption/tink-keyset.json
```

### Disabling encryption

To disable encryption after it was enabled, you must set the disable flag while still providing the keyset file.
This allows the server to decrypt all existing data:

```yaml
crow_encryption_enabled: true
crow_encryption_disable: true
crow_encryption_tink_keyset_file: /path/to/local/keyset.json
```

After the server has decrypted all data (check logs for confirmation), you can remove the encryption configuration.
