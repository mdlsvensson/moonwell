using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using War3Net.Build.Environment;
using WorldEditorCamera = War3Net.Build.Environment.Camera;

namespace Moonwell.Build.Tests
{
    public sealed class PlaceWriterTests : IDisposable
    {
        private readonly TestFolders folders = new TestFolders();

        public void Dispose()
        {
            folders.Dispose();
        }

        [Fact]
        public void writes_regions_in_order()
        {
            var project = Project();
            var regions = Regions(Region("Bank", -256, -128, 64, 32.5f), Region("Gate", 0, 0, 1024, 512));

            var path = PlaceWriter.WriteAreas(project, regions);

            Assert.Equal(Path.Combine(project.GeneratedFolder, "AreaData.g.cs"), path);
            var text = File.ReadAllText(path);
            Assert.Contains("namespace Moonwell.Generated", text);
            Assert.Contains("public static class AreaData", text);
            Assert.Contains("public static string[] Names() => new[] { \"Bank\", \"Gate\" };", text);
            Assert.Contains("public static float[] Rects() => new[] { -256f, -128f, 64f, 32.5f, 0f, 0f, 1024f, 512f };", text);
            Assert.Empty(Syntax(text).GetDiagnostics());
        }

        [Fact]
        public void escapes_names()
        {
            var project = Project();
            const string Name = "Shop \"A\" \\ B";

            var path = PlaceWriter.WriteAreas(project, Regions(Region(Name, 0, 0, 32, 32)));

            var tree = Syntax(File.ReadAllText(path));
            Assert.Empty(tree.GetDiagnostics());
            var literals = tree.GetRoot().DescendantNodes().OfType<LiteralExpressionSyntax>()
                .Where(literal => literal.IsKind(SyntaxKind.StringLiteralExpression))
                .Select(literal => literal.Token.ValueText)
                .ToList();
            Assert.Contains(Name, literals);
        }

        [Fact]
        public void no_regions_file()
        {
            var project = Project();

            var path = PlaceWriter.WriteAreas(project, null);

            var text = File.ReadAllText(path);
            Assert.Contains("public static string[] Names() => new string[0];", text);
            Assert.Contains("public static float[] Rects() => new float[0];", text);
            Assert.Empty(Syntax(text).GetDiagnostics());
        }

        [Fact]
        public void duplicate_name_fails()
        {
            var project = Project();
            var regions = Regions(Region("Bank", 0, 0, 32, 32), Region("Gate", 0, 0, 32, 32), Region("Bank", 64, 64, 96, 96));

            var error = Assert.Throws<BuildException>(() => PlaceWriter.WriteAreas(project, regions));

            Assert.Contains("two regions named 'Bank'", error.Message);
        }

        [Fact]
        public void delete_removes_the_file()
        {
            var project = Project();
            var path = PlaceWriter.WriteAreas(project, null);

            PlaceWriter.DeleteAreas(project);
            PlaceWriter.DeleteAreas(project);

            Assert.False(File.Exists(path));
        }

        [Fact]
        public void writes_cameras_in_order()
        {
            var project = Project();
            var cameras = Cameras(
                MapCamerasFormatVersion.v0,
                false,
                NewCamera("Intro", 100, -200, zOffset: 10, rotation: 90, angleOfAttack: 304, targetDistance: 1650, roll: 0, fieldOfView: 70, farClippingPlane: 5000),
                NewCamera("Boss", 0.5f, 64, zOffset: 1, rotation: 2, angleOfAttack: 3, targetDistance: 4, roll: 5, fieldOfView: 6, farClippingPlane: 7));

            var path = PlaceWriter.WriteInput(project, cameras, false);

            Assert.Equal(Path.Combine(project.GeneratedFolder, "InputData.g.cs"), path);
            var text = File.ReadAllText(path);
            Assert.Contains("namespace Moonwell.Generated", text);
            Assert.Contains("public static class InputData", text);
            Assert.Contains("public static string[] CameraNames() => new[] { \"Intro\", \"Boss\" };", text);

            // Per camera: target x and y, the pair count, the pairs in the order of War3Net's CreateCameras (z offset 6, rotation 5, angle of attack 2,
            // distance 0, roll 4, field of view 3, far z 1), then -1 for the camera type an old format does not have.
            Assert.Contains(
                "public static float[] Cameras() => new[] { "
                + "100f, -200f, 7f, 6f, 10f, 5f, 90f, 2f, 304f, 0f, 1650f, 4f, 0f, 3f, 70f, 1f, 5000f, -1f, "
                + "0.5f, 64f, 7f, 6f, 1f, 5f, 2f, 2f, 3f, 0f, 4f, 4f, 5f, 3f, 6f, 1f, 7f, -1f };",
                text);
            Assert.Empty(Syntax(text).GetDiagnostics());
        }

        [Fact]
        public void a_new_format_camera_has_near_z_and_the_local_angles()
        {
            var project = Project();
            var cameras = Cameras(MapCamerasFormatVersion.v0, true, NewCamera("A", 1, 2, nearClippingPlane: 16, localPitch: 8, localYaw: 9, localRoll: 10));

            var text = File.ReadAllText(PlaceWriter.WriteInput(project, cameras, false));

            Assert.Contains("new[] { 1f, 2f, 11f, 6f, 0f, 5f, 0f, 2f, 0f, 0f, 0f, 4f, 0f, 3f, 0f, 1f, 0f, 7f, 16f, 8f, 8f, 9f, 9f, 10f, 10f, -1f };", text);
        }

        [Fact]
        public void a_v3_camera_has_depth_of_field_absolute_z_and_its_type()
        {
            var project = Project();
            var cameras = Cameras(
                MapCamerasFormatVersion.v3,
                true,
                NewCamera("A", 1, 2, zOffset: 10, nearClippingPlane: 16, localPitch: 8, localYaw: 9, localRoll: 10, dofDistance: 11, dofScale: 12, posAbsoluteZ: 13, cameraType: 2));

            var text = File.ReadAllText(PlaceWriter.WriteInput(project, cameras, false));

            Assert.Contains(
                "new[] { 1f, 2f, 14f, 6f, 10f, 5f, 0f, 2f, 0f, 0f, 0f, 4f, 0f, 3f, 0f, 1f, 0f, 7f, 16f, 8f, 8f, 9f, 9f, 10f, 10f, 11f, 11f, 12f, 12f, 13f, 13f, 2f };",
                text);
        }

        [Fact]
        public void mouse_flag_written()
        {
            var project = Project();

            var on = File.ReadAllText(PlaceWriter.WriteInput(project, null, true));
            var off = File.ReadAllText(PlaceWriter.WriteInput(project, null, false));

            Assert.Contains("public const bool Mouse = true;", on);
            Assert.Contains("public const bool Mouse = false;", off);
            Assert.Empty(Syntax(on).GetDiagnostics());
        }

        [Fact]
        public void no_cameras_file()
        {
            var project = Project();

            var text = File.ReadAllText(PlaceWriter.WriteInput(project, null, false));

            Assert.Contains("public static string[] CameraNames() => new string[0];", text);
            Assert.Contains("public static float[] Cameras() => new float[0];", text);
            Assert.Empty(Syntax(text).GetDiagnostics());
        }

        [Fact]
        public void camera_names_are_escaped()
        {
            var project = Project();
            const string Name = "Cam \"A\" \\ B";

            var path = PlaceWriter.WriteInput(project, Cameras(MapCamerasFormatVersion.v0, false, NewCamera(Name, 0, 0)), false);

            var tree = Syntax(File.ReadAllText(path));
            Assert.Empty(tree.GetDiagnostics());
            var literals = tree.GetRoot().DescendantNodes().OfType<LiteralExpressionSyntax>()
                .Where(literal => literal.IsKind(SyntaxKind.StringLiteralExpression))
                .Select(literal => literal.Token.ValueText)
                .ToList();
            Assert.Contains(Name, literals);
        }

        [Fact]
        public void duplicate_camera_name_fails()
        {
            var project = Project();
            var cameras = Cameras(MapCamerasFormatVersion.v0, false, NewCamera("Intro", 0, 0), NewCamera("Boss", 0, 0), NewCamera("Intro", 1, 1));

            var error = Assert.Throws<BuildException>(() => PlaceWriter.WriteInput(project, cameras, false));

            Assert.Contains("two cameras named 'Intro'", error.Message);
        }

        [Fact]
        public void delete_input_removes_the_file()
        {
            var project = Project();
            var path = PlaceWriter.WriteInput(project, null, false);

            PlaceWriter.DeleteInput(project);
            PlaceWriter.DeleteInput(project);

            Assert.False(File.Exists(path));
        }

        private MapProject Project()
        {
            var folder = folders.TemporaryFolderOutsideRepository();
            Directory.CreateDirectory(Path.Combine(folder, "map"));
            File.WriteAllText(Path.Combine(folder, "map", "war3map.w3i"), string.Empty);
            File.WriteAllText(Path.Combine(folder, "Test.csproj"), "<Project />");
            return MapProject.Find(folder);
        }

        private static MapRegions Regions(params Region[] regions)
        {
            var map = new MapRegions(MapRegionsFormatVersion.v5);
            map.Regions.AddRange(regions);
            return map;
        }

        private static Region Region(string name, float left, float bottom, float right, float top)
        {
            return new Region { Name = name, Left = left, Bottom = bottom, Right = right, Top = top };
        }

        private static MapCameras Cameras(MapCamerasFormatVersion version, bool newFormat, params WorldEditorCamera[] cameras)
        {
            var map = new MapCameras(version, newFormat);
            map.Cameras.AddRange(cameras);
            return map;
        }

        private static WorldEditorCamera NewCamera(
            string name,
            float x,
            float y,
            float zOffset = 0,
            float rotation = 0,
            float angleOfAttack = 0,
            float targetDistance = 0,
            float roll = 0,
            float fieldOfView = 0,
            float farClippingPlane = 0,
            float nearClippingPlane = 0,
            float localPitch = 0,
            float localYaw = 0,
            float localRoll = 0,
            float dofDistance = 0,
            float dofScale = 0,
            float posAbsoluteZ = 0,
            int cameraType = 0)
        {
            return new WorldEditorCamera
            {
                Name = name,
                TargetPosition = new System.Numerics.Vector2(x, y),
                ZOffset = zOffset,
                Rotation = rotation,
                AngleOfAttack = angleOfAttack,
                TargetDistance = targetDistance,
                Roll = roll,
                FieldOfView = fieldOfView,
                FarClippingPlane = farClippingPlane,
                NearClippingPlane = nearClippingPlane,
                LocalPitch = localPitch,
                LocalYaw = localYaw,
                LocalRoll = localRoll,
                DofDistance = dofDistance,
                DofScale = dofScale,
                PosAbsoluteZ = posAbsoluteZ,
                CameraType = cameraType,
            };
        }

        private static SyntaxTree Syntax(string text)
        {
            return CSharpSyntaxTree.ParseText(text);
        }
    }
}
