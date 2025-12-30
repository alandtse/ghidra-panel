GRADLE?=./gradlew
SQLITE?=sqlite3

.PHONY: build
build:
ifeq ($(OS),Windows_NT)
	.\gradlew.bat jar
else
	$(GRADLE) jar
endif

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
