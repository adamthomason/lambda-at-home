#!/usr/bin/env python3
from setuptools import setup, find_packages
import os

# Read the contents of your README file (if you have one)
this_directory = os.path.abspath(os.path.dirname(__file__))
try:
    with open(os.path.join(this_directory, "README.md"), encoding="utf-8") as f:
        long_description = f.read()
except FileNotFoundError:
    long_description = "VM Runtime Package"

setup(
    name="lambda-at-home-3.13-runtime",
    version="1.0.0",
    author="Adam Thomason",
    author_email="adam.thomason@a17n.co.uk",
    description="Lambda at home Python 3.13 runtime",
    long_description=long_description,
    long_description_content_type="text/markdown",
    packages=find_packages(),
    python_requires=">=3.13",
    entry_points={
        "console_scripts": [
            "runtime=runtime.main:main",
        ],
    },
    package_data={
        "runtime": ["*.py"],
    },
    include_package_data=True,
)
