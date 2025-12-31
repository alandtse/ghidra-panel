package re.mkw.srejaas;

import com.google.protobuf.Empty;
import ghidra.framework.Application;
import ghidra.framework.store.local.LocalFileSystem;
import ghidra.server.RepositoryManager;
import ghidra.server.UserManager;
import ghidra.server.remote.GhidraServer;
import ghidra.util.exception.DuplicateNameException;
import io.grpc.stub.StreamObserver;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.Logger;
import re.mkw.srejaas.proto.*;
import re.mkw.srejaas.reflect.GhidraServerSupport;
import re.mkw.srejaas.reflect.LocalFileSystemSupport;
import re.mkw.srejaas.reflect.RepositoryManagerSupport;
import re.mkw.srejaas.reflect.RepositorySupport;

import javax.security.auth.login.FailedLoginException;
import java.io.File;
import java.io.IOException;
import java.util.Arrays;

public class GrpcImpl extends GhidraGrpc.GhidraImplBase {
  private final RepositoryManager repositoryManager;
  private final UserManager userManager;
  private final Logger log;

  public GrpcImpl() {
    GhidraServer ghidraServer = GhidraServerSupport.getGhidraServer();
    repositoryManager = GhidraServerSupport.getRepositoryManager(ghidraServer);
    userManager = repositoryManager.getUserManager();
    log = LogManager.getLogger(GrpcImpl.class);
  }

  @Override
  public void getVersion(Empty request, StreamObserver<Version> responseObserver) {
    responseObserver.onNext(buildVersion());
    responseObserver.onCompleted();
  }

  @Override
  public void getRepositories(Empty request, StreamObserver<GetRepositoriesReply> responseObserver) {
    String repositoriesDir = RepositoryManagerSupport.getRootDir(repositoryManager).getAbsolutePath();
    GetRepositoriesReply.Builder builder = GetRepositoriesReply.newBuilder()
        .setVersion(buildVersion())
        .setRepositoriesDir(repositoriesDir);
    for (String name : RepositoryManagerSupport.getRepositoryNames(repositoryManager)) {
      ghidra.server.Repository repository = RepositoryManagerSupport.getRepository(repositoryManager, name);
      builder.addRepositories(buildRepository(repository));
    }
    for (String user : userManager.getUsers()) {
      builder.addUsers(buildUser(user));
    }
    responseObserver.onNext(builder.build());
    responseObserver.onCompleted();
  }

  @Override
  public void getRepository(GetRepositoryRequest request, StreamObserver<GetRepositoryReply> responseObserver) {
    String repositoriesDir = RepositoryManagerSupport.getRootDir(repositoryManager).getAbsolutePath();
    GetRepositoryReply.Builder builder = GetRepositoryReply.newBuilder()
        .setVersion(buildVersion())
        .setRepositoriesDir(repositoriesDir);
    ghidra.server.Repository repository = RepositoryManagerSupport.getRepository(repositoryManager, request.getRepository());
    if (repository != null) {
      builder.setRepository(buildRepository(repository));
    }
    for (String user : userManager.getUsers()) {
      builder.addUsers(buildUser(user));
    }
    responseObserver.onNext(builder.build());
    responseObserver.onCompleted();
  }

  @Override
  public void getRepositoryUser(GetRepositoryUserRequest request, StreamObserver<GetRepositoryUserReply> responseObserver) {
    ghidra.server.Repository repository = RepositoryManagerSupport.getRepository(repositoryManager, request.getRepository());
    if (repository == null) {
      responseObserver.onError(io.grpc.Status.NOT_FOUND.withDescription("Repository not found").asRuntimeException());
      return;
    }
    ghidra.framework.remote.User user = repository.getUser(request.getUsername());
    GetRepositoryUserReply.Builder builder = GetRepositoryUserReply.newBuilder();
    if (user != null) {
      builder.setResult(buildUserWithPermission(user));
    }
    responseObserver.onNext(builder.build());
    responseObserver.onCompleted();
  }

  @Override
  public void getUsers(Empty request, StreamObserver<GetUsersReply> responseObserver) {
    GetUsersReply.Builder builder = GetUsersReply.newBuilder();
    for (String user : userManager.getUsers()) {
      builder.addUsers(buildUser(user));
    }
    responseObserver.onNext(builder.build());
    responseObserver.onCompleted();
  }

  @Override
  public void addUser(AddUserRequest request, StreamObserver<Empty> responseObserver) {
    log.info("Adding user: {}", request.getUsername());
    try {
      userManager.addUser(request.getUsername());
      responseObserver.onNext(Empty.getDefaultInstance());
      responseObserver.onCompleted();
    } catch (DuplicateNameException e) {
      responseObserver.onError(io.grpc.Status.ALREADY_EXISTS.withDescription(e.getMessage()).asRuntimeException());
    } catch (IOException e) {
      responseObserver.onError(io.grpc.Status.INTERNAL.withDescription(e.getMessage()).asRuntimeException());
    }
  }

  @Override
  public void removeUser(RemoveUserRequest request, StreamObserver<Empty> responseObserver) {
    log.info("Removing user: {}", request.getUsername());
    try {
      if (userManager.removeUser(request.getUsername())) {
        responseObserver.onNext(Empty.getDefaultInstance());
        responseObserver.onCompleted();
      } else {
        responseObserver.onError(io.grpc.Status.NOT_FOUND.withDescription("User not found").asRuntimeException());
      }
    } catch (IOException e) {
      responseObserver.onError(io.grpc.Status.INTERNAL.withDescription(e.getMessage()).asRuntimeException());
    }
  }

  @Override
  public void setUserPermission(SetUserPermissionRequest request, StreamObserver<Empty> responseObserver) {
    log.info("Setting user permission: {} {} {}", request.getUsername(), request.getRepository(), request.getPermission());
    try {
      ghidra.server.Repository repository = RepositoryManagerSupport.getRepository(repositoryManager, request.getRepository());
      if (repository == null) {
        responseObserver.onError(io.grpc.Status.NOT_FOUND.withDescription("Repository not found").asRuntimeException());
        return;
      }
      if (request.getPermission() == Permission.NONE) {
        if (RepositorySupport.removeUser(repository, request.getUsername())) {
          responseObserver.onNext(Empty.getDefaultInstance());
          responseObserver.onCompleted();
        } else {
          responseObserver.onError(io.grpc.Status.NOT_FOUND.withDescription("User not found").asRuntimeException());
        }
        return;
      }
      // Temporary hack: some users may not have been added to the UserManager
      if (!userManager.isValidUser(request.getUsername())) {
        try {
          userManager.addUser(request.getUsername());
        } catch (DuplicateNameException ignored) {
        }
      }
      if (RepositorySupport.setUserPermission(repository, request.getUsername(), request.getPermission().getNumber())) {
        responseObserver.onNext(Empty.getDefaultInstance());
        responseObserver.onCompleted();
      } else {
        responseObserver.onError(io.grpc.Status.NOT_FOUND.withDescription("User not found").asRuntimeException());
      }
    } catch (IOException e) {
      responseObserver.onError(io.grpc.Status.INTERNAL.withDescription(e.getMessage()).asRuntimeException());
    }
  }

  @Override
  public void authenticateUser(AuthenticateUserRequest request, StreamObserver<AuthenticateUserReply> responseObserver) {
    try {
      AuthenticateUserReply.Builder builder = AuthenticateUserReply.newBuilder();
      // Ensure case-insensitivity
      String username = request.getUsername();
      for (String user : userManager.getUsers()) {
        if (user.equalsIgnoreCase(request.getUsername())) {
          username = user;
          builder.setUsername(user);
          break;
        }
      }
      char[] password = request.getPassword().toCharArray();
      try {
        userManager.authenticateUser(username, password);
        builder.setSuccess(true);
      } catch (FailedLoginException e) {
        builder.setSuccess(false).setMessage(e.getMessage());
      } finally {
        Arrays.fill(password, '\0');
      }
      responseObserver.onNext(builder.build());
      responseObserver.onCompleted();
    } catch (IOException e) {
      responseObserver.onError(io.grpc.Status.INTERNAL.withDescription(e.getMessage()).asRuntimeException());
    }
  }

  @Override
  public void deleteRepository(DeleteRepositoryRequest request, StreamObserver<Empty> responseObserver) {
    log.warn("Deleting repository: {}", request.getRepository());
    try {
      RepositoryManagerSupport.deleteRepository(repositoryManager, request.getRepository());
    } catch (IOException e) {
      responseObserver.onError(io.grpc.Status.INTERNAL.withDescription(e.getMessage()).asRuntimeException());
    }
    responseObserver.onNext(Empty.getDefaultInstance());
    responseObserver.onCompleted();
  }

  private Version buildVersion() {
    String ghidraVersion = Application.getApplicationVersion();
    String panelVersion = getClass().getPackage().getImplementationVersion();
    if (panelVersion == null) {
      panelVersion = "unknown";
    }
    return Version.newBuilder()
        .setGhidraVersion(ghidraVersion)
        .setGhidraPanelVersion(panelVersion)
        .build();
  }

  private User buildUser(String name) {
    boolean hasPassword = userManager.getPasswordExpiration(name) != 0;
    return User.newBuilder()
        .setUsername(name)
        .setHasPassword(hasPassword)
        .build();
  }

  private UserWithPermission buildUserWithPermission(ghidra.framework.remote.User user) {
    return UserWithPermission.newBuilder()
        .setUser(buildUser(user.getName()))
        .setPermission(Permission.forNumber(user.getPermissionType()))
        .build();
  }

  private Repository buildRepository(ghidra.server.Repository repository) {
    LocalFileSystem fileSystem = RepositorySupport.getFileSystem(repository);
    File root = LocalFileSystemSupport.getRoot(fileSystem);
    ghidra.framework.remote.User[] users = RepositorySupport.getRepositoryUsers(repository);
    
    // Build repository stats
    RepositoryStats stats = buildRepositoryStats(root, users.length);
    
    return Repository.newBuilder()
        .setName(repository.getName())
        .setPath(root.getAbsolutePath())
        .setAnonymousAccessAllowed(repository.anonymousAccessAllowed())
        .addAllUsers(
            Arrays.stream(users)
                .map(this::buildUserWithPermission)
                .toList()
        )
        .setStats(stats)
        .build();
  }

  private RepositoryStats buildRepositoryStats(File repoRoot, int userCount) {
    RepositoryStats.Builder builder = RepositoryStats.newBuilder();
    
    builder.setUserCount(userCount);
    
    if (repoRoot != null && repoRoot.exists()) {
      // Get creation/modification times and calculate size
      long createdTime = repoRoot.lastModified(); // Best approximation for creation
      long lastModifiedTime = repoRoot.lastModified();
      long totalSize = 0;
      int fileCount = 0;
      
      // Recursively calculate size and count files
      try {
        DirStats stats = calculateDirStats(repoRoot);
        totalSize = stats.size;
        fileCount = stats.fileCount;
        lastModifiedTime = Math.max(lastModifiedTime, stats.lastModified);
      } catch (Exception e) {
        log.warn("Failed to calculate repository stats for " + repoRoot.getAbsolutePath(), e);
      }
      
      builder.setSizeBytes(totalSize);
      builder.setCreatedTime(createdTime);
      builder.setLastModifiedTime(lastModifiedTime);
      builder.setFileCount(fileCount);
    }
    
    return builder.build();
  }

  private static class DirStats {
    long size = 0;
    int fileCount = 0;
    long lastModified = 0;
  }

  private DirStats calculateDirStats(File dir) {
    DirStats stats = new DirStats();
    
    if (!dir.exists() || !dir.isDirectory()) {
      return stats;
    }
    
    File[] files = dir.listFiles();
    if (files == null) {
      return stats;
    }
    
    for (File file : files) {
      if (file.isFile()) {
        stats.size += file.length();
        stats.fileCount++;
        stats.lastModified = Math.max(stats.lastModified, file.lastModified());
      } else if (file.isDirectory()) {
        DirStats subStats = calculateDirStats(file);
        stats.size += subStats.size;
        stats.fileCount += subStats.fileCount;
        stats.lastModified = Math.max(stats.lastModified, subStats.lastModified);
      }
    }
    
    return stats;
  }
}
