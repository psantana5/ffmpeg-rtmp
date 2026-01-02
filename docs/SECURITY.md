# Security Policy

## Supported Versions

We actively support the following versions:

| Version | Supported          |
| ------- | ------------------ |
| main    | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please report it by following these steps:

### How to Report

1. **Do NOT open a public issue** - This could put all users at risk
2. **Email the maintainers directly** at the email listed in the repository
3. **Provide detailed information** including:
   - Description of the vulnerability
   - Steps to reproduce the issue
   - Potential impact
   - Suggested fix (if you have one)

### What to Include

- Type of vulnerability (e.g., SQL injection, XSS, authentication bypass)
- Full paths of source file(s) related to the vulnerability
- Location of the affected source code (tag/branch/commit or direct URL)
- Any special configuration required to reproduce the issue
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit it

### Response Timeline

- **Initial Response**: Within 48 hours of report
- **Status Update**: Within 7 days with initial assessment
- **Fix Timeline**: Varies by severity
  - Critical: 1-7 days
  - High: 7-30 days
  - Medium: 30-90 days
  - Low: Best effort

### What to Expect

1. We will confirm receipt of your vulnerability report
2. We will investigate and validate the issue
3. We will work on a fix and coordinate disclosure timeline with you
4. We will release a security advisory when the fix is available
5. We will credit you for the discovery (unless you prefer to remain anonymous)

## Security Best Practices

### When Running This Project

1. **Network Security**
   - Run the stack behind a firewall
   - Do not expose ports directly to the internet without proper authentication
   - Use strong passwords for Grafana and other services

2. **Container Security**
   - Keep Docker and Docker Compose up to date
   - Regularly update base images
   - Scan images for vulnerabilities using the built-in Trivy scans

3. **Secrets Management**
   - Never commit secrets to version control
   - Use environment variables or Docker secrets for sensitive data
   - Rotate credentials regularly

4. **Access Control**
   - Limit who has access to the monitoring stack
   - Use role-based access control in Grafana
   - Monitor access logs regularly

5. **Updates**
   - Keep Python dependencies up to date
   - Enable Dependabot for automated updates
   - Review and apply security patches promptly

## Known Security Considerations

### RAPL Exporter

The RAPL exporter requires access to `/sys/class/powercap` which provides system power information. This is read-only and poses minimal security risk, but users should be aware of the privilege requirements.

### Docker Socket Access

Some exporters (like docker_stats) require access to the Docker socket. This provides significant privileges and should only be used in trusted environments.

### Prometheus and Grafana

- Default credentials should be changed immediately
- Enable authentication for all services
- Use HTTPS in production environments
- Restrict network access to monitoring services

## Security Scanning

This project uses:

- **Trivy** - Container vulnerability scanning in CI/CD
- **Dependabot** - Automated dependency updates
- **CodeQL** - Static code analysis (when enabled)
- **Ruff** - Python linting with security rules

## Vulnerability Disclosure Policy

We follow responsible disclosure practices:

1. Security researchers are encouraged to report vulnerabilities privately
2. We will work with you to understand and validate the issue
3. We will develop and test a fix
4. We will coordinate disclosure timing
5. We will publicly acknowledge your contribution (if desired)

## Additional Resources

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [Docker Security Best Practices](https://docs.docker.com/engine/security/)
- [Prometheus Security](https://prometheus.io/docs/operating/security/)
- [Grafana Security](https://grafana.com/docs/grafana/latest/administration/security/)

## Contact

For security concerns, please check the repository for current maintainer contact information.

---

Last updated: 2024-12-29
