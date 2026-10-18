namespace Moonwell.Build
{
    /// <summary>Builds a map project into a map archive.</summary>
    public static class MapBuild
    {
        /// <summary>Runs every step of the build, in order.</summary>
        /// <param name="path">A project folder, or the path of its <c>.csproj</c> file.</param>
        /// <param name="options">What kind of build to make.</param>
        /// <param name="log">Where the build's progress, notes and <c>dotnet build</c> output go.</param>
        /// <returns>The full path of the map archive.</returns>
        /// <exception cref="BuildException">A step failed in a way the modder can act on.</exception>
        public static string Run(string path, BuildOptions options, TextWriter log)
        {
            var project = MapProject.Find(path);
            log.WriteLine($"moonwell: {project.Name} ({(options.Release ? "release" : "development")} build)");

            var targetPath = DotnetBuild.Run(project, options.Release, log);
            PackageSources.Check(project);
            var settings = MapSettingsReader.Read(targetPath, project);
            var map = SourceMap.Open(project, settings, log);

            // The reference decides, not a Moonwell.Objects.dll an earlier build left beside the assembly.
            if (Declarations.UsesObjects(targetPath))
            {
                var objectsDll = Path.Combine(Path.GetDirectoryName(Path.GetFullPath(targetPath))!, Declarations.ObjectsAssemblyName + ".dll");
                BuildWithObjects(project, settings, map, targetPath, objectsDll, options, log);
            }
            else
            {
                BuildWithoutObjects(project, settings, map, targetPath, options, log);
            }

            try
            {
                Archive.Pack(project.StagedFolder, project.ArchivePath);
            }
            catch (Exception error) when (error is IOException or UnauthorizedAccessException)
            {
                throw CannotWrite(project, project.ArchivePath, error);
            }

            log.WriteLine($@"moonwell: built build\{project.Name}.w3x");
            return project.ArchivePath;
        }

        // The map uses Moonwell.Objects: its declarations, ids, Lua, registry and object files are part of the build.
        private static void BuildWithObjects(
            MapProject project,
            MapSettings settings,
            War3Net.Build.Map map,
            string targetPath,
            string objectsDll,
            BuildOptions options,
            TextWriter log)
        {
            var data = GameData.Of(settings.DataSet, settings.DataVersion);
            var standard = StandardData.Read(objectsDll);
            var declarations = Declarations.Read(targetPath, project, standard, data.ValueSet);
            BalanceCheck.Run(declarations, standard, data);

            var ids = ObjectIds.Assign(project, declarations.Objects, standard.AllIds(), MapObjectIds.Defined(project), MapObjectIds.Placed(map), log);
            EntryWriter.Write(project, settings, true);
            RegistryWriter.Write(project, declarations.Used, declarations.Objects, ids);
            LuaCompiler.Compile(project, map, targetPath, options.Release, declarations, ids, standard, data.ValueSet);
            StageWith(project, () => Stage.Write(project, map, declarations.Objects, ids, declarations.StandardClassIds, standard, log));
        }

        // The map does not reference Moonwell.Objects: its own object files are used as they are.
        private static void BuildWithoutObjects(MapProject project, MapSettings settings, War3Net.Build.Map map, string targetPath, BuildOptions options, TextWriter log)
        {
            // A registry or stubs left by an earlier build would not compile any more.
            var registry = Path.Combine(project.GeneratedFolder, "ObjectRegistry.g.cs");
            if (File.Exists(registry))
            {
                File.Delete(registry);
            }

            ObjectStubs.Delete(project);

            EntryWriter.Write(project, settings);
            LuaCompiler.Compile(project, map, targetPath, options.Release);
            StageWith(project, () => Stage.WriteWithoutObjects(project, map, log));
        }

        private static void StageWith(MapProject project, Action stage)
        {
            try
            {
                stage();
            }
            catch (Exception error) when (error is IOException or UnauthorizedAccessException)
            {
                throw CannotWrite(project, project.StagedFolder, error);
            }
        }

        private static BuildException CannotWrite(MapProject project, string path, Exception error)
        {
            var relative = Path.GetRelativePath(project.Folder, path);
            return new BuildException($"cannot write {relative}: {error.Message} (is the game or World Editor using it?)");
        }
    }
}
