namespace Moonwell.Build
{
    /// <summary>Builds a map project into a map archive.</summary>
    public static class MapBuild
    {
        /// <summary>The damage package's assembly: a map that references it gets it installed by its Main.</summary>
        private const string DamageAssemblyName = "Moonwell.Damage";

        /// <summary>The interactions package's assembly: a map that references it gets the table of its tree types in its object Lua.</summary>
        private const string InteractionsAssemblyName = "Moonwell.Interactions";

        /// <summary>The terrain package's assembly: a map that references it gets its grid written from <c>war3map.w3e</c> and installed by its Main.</summary>
        private const string TerrainAssemblyName = "Moonwell.Terrain";

        /// <summary>The areas package's assembly: a map that references it gets its World Editor regions written from <c>war3map.w3r</c>, their names in its code checked, and the package installed by its Main.</summary>
        private const string AreasAssemblyName = "Moonwell.Areas";

        /// <summary>The input package's assembly: a map that references it gets World Editor's cameras written from <c>war3map.w3c</c>, their names in its code checked, and the package installed by its Main, listening for the mouse only when the code names <c>Mouse</c> or <c>IMouseWatch</c>.</summary>
        private const string InputAssemblyName = "Moonwell.Input";

        /// <summary>The physics package's assembly: a map that references it gets its placed bridges written from <c>war3map.doo</c> and installed by its Main.</summary>
        private const string PhysicsAssemblyName = "Moonwell.Physics";

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

            var ids = ObjectIds.Assign(declarations.Objects, standard.AllIds(), MapObjectIds.Defined(project));
            EntryWriter.Write(project, settings, true, Installs(project, map, targetPath, BridgeWriter.Declared(declarations.Objects, ids), standard.AllIds(), log));
            RegistryWriter.Write(project, declarations.Used, declarations.Objects, ids);
            LuaCompiler.Compile(project, map, targetPath, options.Release, declarations, ids, standard, data.ValueSet, Declarations.References(targetPath, InteractionsAssemblyName));
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

            // The table of tree types is made from the standard destructable data, which only Moonwell.Objects carries.
            if (Declarations.References(targetPath, InteractionsAssemblyName))
            {
                log.WriteLine("warning: the map uses Moonwell.Interactions without Moonwell.Objects: IsTree needs Moonwell.Objects (its standard data makes the table of tree types), so it is false for every destructable");
            }

            // The build reads no declarations, but a unit's Level is still set only in a declared class's constructor.
            Declarations.CheckSetters(targetPath);

            EntryWriter.Write(project, settings, false, Installs(project, map, targetPath, new List<BridgeWriter.OwnType>(), null, log));
            LuaCompiler.Compile(project, map, targetPath, options.Release);
            StageWith(project, () => Stage.WriteWithoutObjects(project, map, log));
        }

        // What the map's Main installs before anything registers: each package the map references, in this order. The terrain's grid, the
        // areas' regions, the input's cameras and the physics' bridges are written here too, from the map's ground, regions, cameras and
        // placed destructables (with the destructable types the map declares and the standard ids, when it uses Moonwell.Objects),
        // and a file an earlier build wrote is removed when the map no longer uses the package.
        private static List<string> Installs(MapProject project, War3Net.Build.Map map, string targetPath, IReadOnlyList<BridgeWriter.OwnType> declared, IReadOnlySet<string>? standardIds, TextWriter log)
        {
            var installs = new List<string>();
            if (Declarations.References(targetPath, DamageAssemblyName))
            {
                installs.Add("global::Moonwell.DamageSystem.Install();");
            }

            // The physics package reads the ground through the terrain package, so a map that uses physics but never names Terrain
            // (its assembly then has no reference to it) needs the grid too.
            var physics = Declarations.References(targetPath, PhysicsAssemblyName);
            if (Declarations.References(targetPath, TerrainAssemblyName) || physics)
            {
                // The fork's W2 fix refuses a map without a war3map.w3e, so there is always one here.
                TerrainWriter.Write(project, map.Environment!);
                const string Data = "global::Moonwell.Generated.TerrainData";
                installs.Add($"global::Moonwell.Terrain.Install({Data}.Columns, {Data}.Rows, {Data}.Left, {Data}.Bottom, {Data}.Corners());");
            }
            else
            {
                TerrainWriter.Delete(project);
            }

            if (Declarations.References(targetPath, AreasAssemblyName))
            {
                // A map with no war3map.w3r has no regions: Regions is null, and the arrays are empty.
                PlaceWriter.WriteAreas(project, map.Regions);
                PlaceNames.Check(project, "AreaShape.Region", map.Regions?.Regions.Select(region => region.Name).ToList() ?? new List<string>(), "region");
                installs.Add("global::Moonwell.AreaSystem.Install(global::Moonwell.Generated.AreaData.Names(), global::Moonwell.Generated.AreaData.Rects());");
            }
            else
            {
                PlaceWriter.DeleteAreas(project);
            }

            if (Declarations.References(targetPath, InputAssemblyName))
            {
                // A map that references the package but never names Mouse or IMouseWatch registers no mouse event: the package listens
                // only when asked. A map with no war3map.w3c has no cameras: Cameras is null, and the arrays are empty.
                var mouse = Declarations.ReferencesType(targetPath, "Moonwell", "Mouse") || Declarations.ReferencesType(targetPath, "Moonwell", "IMouseWatch");
                PlaceWriter.WriteInput(project, map.Cameras, mouse);
                PlaceNames.Check(project, "CameraSetup.Named", map.Cameras?.Cameras.Select(camera => camera.Name).ToList() ?? new List<string>(), "camera");
                installs.Add("global::Moonwell.InputSystem.Install(global::Moonwell.Generated.InputData.Mouse, global::Moonwell.Generated.InputData.CameraNames(), global::Moonwell.Generated.InputData.Cameras());");
            }
            else
            {
                PlaceWriter.DeleteInput(project);
            }

            if (physics)
            {
                // After the terrain: a deck stands on the ground at its bridge's place.
                BridgeWriter.Write(project, map.Doodads, declared, standardIds, log);
                installs.Add("global::Moonwell.Physics.Install(global::Moonwell.Generated.PhysicsData.Bridges(), global::Moonwell.Generated.PhysicsData.Decks());");
            }
            else
            {
                BridgeWriter.Delete(project);
            }

            return installs;
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
