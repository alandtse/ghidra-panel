GRADLE?=./gradlew
SQLITE?=sqlite3

.PHONY: build
build:
ifeq ($(OS),Windows_NT)
	.\gradlew.bat jar
else
	$(GRADLE) jar
endif

install: build
ifndef GHIDRA_SERVER_DIR
	$(error GHIDRA_SERVER_DIR is not defined)
endif
	cp build/libs/ghidra-panel-jaas.jar $(GHIDRA_SERVER_DIR)/ghidra-panel-jaas.jar
	@echo "Installed JAR to $(GHIDRA_SERVER_DIR)/ghidra-panel-jaas.jar"

../test.db:
	$(MAKE) -C .. test.db

.PHONY: integration
integration: ../test.db
	SAMPLE_USER=richard SAMPLE_PASS=richard $(MAVEN) \
 	  -Dexec.mainClass=re.mkw.srejaas.SampleAcn \
 	  -Dexec.classpathScope=test test-compile \
 	  -Djava.security.auth.login.config=test.conf exec:java

.PHONY: clean
clean:
	#$(GRADLE) clean
	rm -rf target
