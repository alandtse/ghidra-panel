package re.mkw.srejaas.reflect;

import ghidra.server.Repository;
import ghidra.server.RepositoryManager;
import ghidra.util.NamingUtilities;
import utilities.util.FileUtilities;

import java.io.File;
import java.io.IOException;
import java.lang.invoke.MethodHandle;
import java.lang.invoke.MethodHandles;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.util.HashMap;
import java.util.Map;

public class RepositoryManagerSupport {
  private static MethodHandle getRootDir;
  private static MethodHandle getRepository;
  private static MethodHandle getRepositoryNames;
  private static MethodHandle getRepositoryMap;

  public static File getRootDir(RepositoryManager mgr) {
    if (getRootDir == null) {
      try {
        Method method = RepositoryManager.class.getDeclaredMethod("getRootDir");
        method.setAccessible(true);
        MethodHandles.Lookup lookup = MethodHandles.privateLookupIn(RepositoryManager.class, MethodHandles.lookup());
        getRootDir = lookup.unreflect(method);
      } catch (NoSuchMethodException | IllegalAccessException e) {
        throw new RuntimeException(e);
      }
    }
    try {
      return (File) getRootDir.invokeExact(mgr);
    } catch (Throwable e) {
      throw new RuntimeException(e);
    }
  }

  public static Repository getRepository(RepositoryManager mgr, String name) {
    if (getRepository == null) {
      try {
        Method method = RepositoryManager.class.getDeclaredMethod("getRepository", String.class);
        method.setAccessible(true);
        MethodHandles.Lookup lookup = MethodHandles.privateLookupIn(RepositoryManager.class, MethodHandles.lookup());
        getRepository = lookup.unreflect(method);
      } catch (NoSuchMethodException | IllegalAccessException e) {
        throw new RuntimeException(e);
      }
    }
    try {
      return (Repository) getRepository.invokeExact(mgr, name);
    } catch (Throwable e) {
      throw new RuntimeException(e);
    }
  }

  public static String[] getRepositoryNames(RepositoryManager mgr) {
    if (getRepositoryNames == null) {
      try {
        Method method = RepositoryManager.class.getDeclaredMethod("getRepositoryNames");
        method.setAccessible(true);
        MethodHandles.Lookup lookup = MethodHandles.privateLookupIn(RepositoryManager.class, MethodHandles.lookup());
        getRepositoryNames = lookup.unreflect(method);
      } catch (NoSuchMethodException | IllegalAccessException e) {
        throw new RuntimeException(e);
      }
    }
    try {
      return (String[]) getRepositoryNames.invokeExact(mgr);
    } catch (Throwable e) {
      throw new RuntimeException(e);
    }
  }

  public static Map<String, Repository> getRepositoryMap(RepositoryManager mgr) {
    if (getRepositoryMap == null) {
      try {
        Field field = RepositoryManager.class.getDeclaredField("repositoryMap");
        field.setAccessible(true);
        MethodHandles.Lookup lookup = MethodHandles.privateLookupIn(RepositoryManager.class, MethodHandles.lookup());
        getRepositoryMap = lookup.unreflectGetter(field);
      } catch (NoSuchFieldException | IllegalAccessException e) {
        throw new RuntimeException(e);
      }
    }
    try {
      return (HashMap<String, Repository>) getRepositoryMap.invokeExact(mgr);
    } catch (Throwable e) {
      throw new RuntimeException(e);
    }
  }

  public static void deleteRepository(RepositoryManager mgr, String name) throws IOException {
    synchronized (mgr) {
      Map<String, Repository> repositoryMap = getRepositoryMap(mgr);
      Repository repository = repositoryMap.get(name);
      if (repository != null) {
        RepositorySupport.deleteRepository(repository);
        File rootDir = getRootDir(mgr);
        File f = new File(rootDir, NamingUtilities.mangle(name));
        if (!FileUtilities.deleteDir(f)) {
          throw new IOException("Failed to remove directory for " + f.getAbsolutePath());
        } else {
          repositoryMap.remove(name);
        }
      }
    }
  }
}
