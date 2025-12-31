package re.mkw.srejaas.reflect;

import ghidra.framework.remote.User;
import ghidra.framework.store.local.LocalFileSystem;
import ghidra.server.Repository;
import ghidra.server.store.RepositoryFolder;

import java.io.IOException;
import java.lang.invoke.MethodHandle;
import java.lang.invoke.MethodHandles;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.util.LinkedHashMap;

public class RepositorySupport {
  private static MethodHandle getUserMap;
  private static MethodHandle setUserPermission;
  private static MethodHandle removeUser;
  private static MethodHandle getRootFolder;
  private static MethodHandle getFileSystem;
  private static MethodHandle getValid;
  private static MethodHandle setValid;
  private static MethodHandle dispose;

  public static User[] getRepositoryUsers(Repository repository) {
    if (getUserMap == null) {
      try {
        Field field = Repository.class.getDeclaredField("userMap");
        field.setAccessible(true);
        MethodHandles.Lookup lookup = MethodHandles.privateLookupIn(Repository.class, MethodHandles.lookup());
        getUserMap = lookup.unreflectGetter(field);
      } catch (NoSuchFieldException | IllegalAccessException e) {
        throw new RuntimeException(e);
      }
    }
    synchronized (repository.getSyncObject()) {
      try {
        // noinspection unchecked
        LinkedHashMap<String, User> userMap = (LinkedHashMap<String, User>) getUserMap.invokeExact(repository);
        return userMap.values().toArray(new User[0]);
      } catch (Throwable e) {
        throw new RuntimeException(e);
      }
    }
  }

  public static boolean setUserPermission(Repository repository, String user, int permission) throws IOException {
    if (setUserPermission == null) {
      try {
        Method method = Repository.class.getDeclaredMethod("setUserPermission", String.class, int.class);
        method.setAccessible(true);
        MethodHandles.Lookup lookup = MethodHandles.privateLookupIn(Repository.class, MethodHandles.lookup());
        setUserPermission = lookup.unreflect(method);
      } catch (NoSuchMethodException | IllegalAccessException e) {
        throw new RuntimeException(e);
      }
    }
    try {
      return (boolean) setUserPermission.invokeExact(repository, user, permission);
    } catch (Throwable e) {
      if (e instanceof IOException) {
        throw (IOException) e;
      }
      throw new RuntimeException(e);
    }
  }

  public static boolean removeUser(Repository repository, String user) throws IOException {
    if (removeUser == null) {
      try {
        Method method = Repository.class.getDeclaredMethod("removeUser", String.class);
        method.setAccessible(true);
        MethodHandles.Lookup lookup = MethodHandles.privateLookupIn(Repository.class, MethodHandles.lookup());
        removeUser = lookup.unreflect(method);
      } catch (NoSuchMethodException | IllegalAccessException e) {
        throw new RuntimeException(e);
      }
    }
    try {
      return (boolean) removeUser.invokeExact(repository, user);
    } catch (Throwable e) {
      if (e instanceof IOException) {
        throw (IOException) e;
      }
      throw new RuntimeException(e);
    }
  }

  public static RepositoryFolder getRootFolder(Repository repository) {
    if (getRootFolder == null) {
      try {
        Field field = Repository.class.getDeclaredField("rootFolder");
        field.setAccessible(true);
        MethodHandles.Lookup lookup = MethodHandles.privateLookupIn(Repository.class, MethodHandles.lookup());
        getRootFolder = lookup.unreflectGetter(field);
      } catch (NoSuchFieldException | IllegalAccessException e) {
        throw new RuntimeException(e);
      }
    }
    try {
      return (RepositoryFolder) getRootFolder.invokeExact(repository);
    } catch (Throwable e) {
      throw new RuntimeException(e);
    }
  }

  public static LocalFileSystem getFileSystem(Repository repository) {
    if (getFileSystem == null) {
      try {
        Field field = Repository.class.getDeclaredField("fileSystem");
        field.setAccessible(true);
        MethodHandles.Lookup lookup = MethodHandles.privateLookupIn(Repository.class, MethodHandles.lookup());
        getFileSystem = lookup.unreflectGetter(field);
      } catch (NoSuchFieldException | IllegalAccessException e) {
        throw new RuntimeException(e);
      }
    }
    try {
      return (LocalFileSystem) getFileSystem.invokeExact(repository);
    } catch (Throwable e) {
      throw new RuntimeException(e);
    }
  }

  public static boolean getValid(Repository repository) {
    if (getValid == null) {
      try {
        Field field = Repository.class.getDeclaredField("valid");
        field.setAccessible(true);
        MethodHandles.Lookup lookup = MethodHandles.privateLookupIn(Repository.class, MethodHandles.lookup());
        getValid = lookup.unreflectGetter(field);
      } catch (NoSuchFieldException | IllegalAccessException e) {
        throw new RuntimeException(e);
      }
    }
    try {
      return (boolean) getValid.invokeExact(repository);
    } catch (Throwable e) {
      throw new RuntimeException(e);
    }
  }

  public static void setValid(Repository repository, boolean valid) {
    if (setValid == null) {
      try {
        Field field = Repository.class.getDeclaredField("valid");
        field.setAccessible(true);
        MethodHandles.Lookup lookup = MethodHandles.privateLookupIn(Repository.class, MethodHandles.lookup());
        setValid = lookup.unreflectSetter(field);
      } catch (NoSuchFieldException | IllegalAccessException e) {
        throw new RuntimeException(e);
      }
    }
    try {
      setValid.invokeExact(repository, valid);
    } catch (Throwable e) {
      throw new RuntimeException(e);
    }
  }

  public static void deleteRepository(Repository repository) {
    // There's a delete method in Repository, but it's unimplemented
    // We'll implement it ourselves by setting the valid flag and calling dispose
    if (dispose == null) {
      try {
        Method method = Repository.class.getDeclaredMethod("dispose");
        method.setAccessible(true);
        MethodHandles.Lookup lookup = MethodHandles.privateLookupIn(Repository.class, MethodHandles.lookup());
        dispose = lookup.unreflect(method);
      } catch (NoSuchMethodException | IllegalAccessException e) {
        throw new RuntimeException(e);
      }
    }
    synchronized (repository.getSyncObject()) {
      if (!getValid(repository)) {
        throw new RuntimeException("Repository already deleted");
      }
      setValid(repository, false);
      try {
        dispose.invokeExact(repository);
      } catch (Throwable e) {
        throw new RuntimeException(e);
      }
    }
  }
}
