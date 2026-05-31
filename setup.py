from setuptools import setup, find_packages

setup(
    name="running-cli",
    version="0.1.0",
    package_dir={"": "src"},
    packages=find_packages(where="src"),
    install_requires=[
        "garminconnect>=0.2.8",
        "requests>=2.31.0",
        "click>=8.1.7",
        "PyYAML>=6.0.1",
    ],
    entry_points={
        "console_scripts": [
            "running-cli=running_cli.main:cli",
        ],
    },
)
