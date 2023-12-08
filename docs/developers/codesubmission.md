# Code Submission Standards

This document outlines the code submission and version control standards, with the code repository being managed based on git.

Branching Model
The code repository adopts a branching model for management, with the main branches as follows:
- master
- develop
- feature/xxxx
- hotfix

There are two primary branches in the repository:
master
This is the product's main code branch, maintaining a stable codebase for external use. Direct work on this master branch is not allowed; instead, work should be done on other specified, independent feature branches (we will talk about this shortly). Not directly committing changes to the master branch is also a common rule in many workflows.

develop
The development branch is the base branch for any new development. When you start a new feature branch, it will be based on development. Additionally, this branch also collects all completed features, waiting to be integrated into the master branch.

These two branches are known as long-term branches. They will exist throughout the entire lifecycle of the project. Other branches, such as those for features or releases, are only temporary. They are created as needed and deleted once they have completed their task.

feature
The feature is used for the development of functionalities and features.

Feature Development
The feature development process is as follows:
1. Start a new feature
   Each time a new feature is started, branch off from develop with a feature/xxx branch. Once the feature is completed, merge it back into develop.
2. Feature submission
   After the feature development is completed, the developer submits a pull request, waits for code review, and after passing the review, merges into the develop branch and deletes the feature/xxx branch.

Managing Releases
Release management is another very important topic in version control.
Creating a Release
When you think the code in the "develop" branch is now a mature release version, which means: first, it includes all new features and necessary fixes; second, it has been thoroughly tested. If both points are satisfied, then it's time to start generating a new release:
1. Merge the develop branch into master.
2. Tag the current master and issue the corresponding release version.

hotfix
Often, just a few hours or days after a release version has been fully tested, some minor errors may be discovered. In this case, we need to create a hotfix branch:
1. Branch off from the main branch to make the hotfix.
2. Submit a pull request and wait for code review.
3. After the review, merge into both the master and develop branches.
