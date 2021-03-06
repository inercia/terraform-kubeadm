all: ci-tests

RED   := \e[31m
GREEN := \e[32m
NC    := \e[0m

ci-setup:
	mkdir -p "$TRAVIS_BUILD_DIR/snaps-cache"

	@echo -e ">>>$(GREEN) Installation stuff specific to the $$E2E_ENV environment... $(NC)"
	[ -n "$$E2E_ENV" ] && make -C "$$E2E_ENV" ci-setup

	@echo -e ">>>$(GREEN) Installing other dependencies, like kubectl, terraform... $(NC)"
	sudo apt-get update && sudo apt-get install -y apt-transport-https
	curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -
	echo "deb https://apt.kubernetes.io/ kubernetes-xenial main" | sudo tee -a /etc/apt/sources.list.d/kubernetes.list
	sudo apt-get update
	sudo apt install -y kubectl kubeadm unzip

	@echo -e ">>>$(GREEN) Installing Terraform... $(NC)"
	wget https://releases.hashicorp.com/terraform/0.11.13/terraform_0.11.13_linux_amd64.zip
	unzip terraform_0.11.13_linux_amd64.zip
	sudo mv terraform /usr/local/bin/

	@echo -e ">>>$(GREEN) Checking if we must setup things for nightly tests... $(NC)"
	@if [ "$$TRAVIS_EVENT_TYPE" = "cron" ] ; then \
		make -C "nightly" ci-setup ; \
	else \
		echo -e ">>>$(GREEN) ... no nightly tests. $(NC)" ; \
	fi

ci-cleanup:
	@[ -n "$$E2E_CLEANUP" ] && \
		echo -e ">>>$(RED) Destroying cluster... $(NC)\n" && \
		make -C $$E2E_ENV ci-cleanup || /bin/true

ci-logs:
	@[ -f $$E2E_ENV/terraform.log ] || echo -e ">>>$(RED) No logs stored at $$E2E_ENV/terraform.log $(NC)"
	@[ -f $$E2E_ENV/terraform.log -a -n "$$IS_CI" ] && \
		echo -e "\n\n\n\n\n\n"                      && \
		cat $$E2E_ENV/terraform.log                 && \
		echo -e "\n\n\n\n\n\n"                      || \
		/bin/true
	@[ -f $$E2E_ENV/terraform.log ]                                          && \
		echo -e ">>>$(RED) Logs available at $$E2E_ENV/terraform.log $(NC)"  || \
		/bin/true

ci-nightly:
	@echo -e ">>>$(GREEN) Checking if we must run the nightly tests... $(NC)"
	@if [ "$$TRAVIS_EVENT_TYPE" = "cron" ] ; then \
		make -C "nightly" ci-tests ; \
	else \
		echo -e ">>>$(GREEN) ... no nightly tests. $(NC)" ; \
	fi

ci-tests-testsuites:
	@echo -e ">>>$(GREEN) Starting tests suites: $(NC)"
	@echo
	@rm -f $$E2E_ENV/terraform.log
	@for d in `pwd`/[0-9]* ; do \
		echo -e ">>>$(GREEN) Running testsuite `basename $$d` $(NC)" ; \
		for i in $$d/[0-9]*-*.sh ; do \
			echo -e ">>>$(GREEN) Running step `basename $$d`/`basename $$i .sh`$(NC)" ; \
			$$i ; \
			if [ $$? -ne 0 ] ; then \
                echo -e ">>>$(RED) FAILED: $$i $(NC)" ; \
				echo -e ">>>" ; \
				echo -e ">>>" ; \
                make ci-cleanup ; \
                echo -e ">>>" ; \
				echo -e ">>>" ; \
                make ci-logs ; \
                exit 1  ; \
            fi ; \
		done ; \
	done
	@echo -e ">>>$(GREEN) All e2e tests have completed$(NC)"


ci-tests: ci-tests-testsuites ci-nightly

