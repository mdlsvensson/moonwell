using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;
using War3Net.Build.Environment;

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

        private static SyntaxTree Syntax(string text)
        {
            return CSharpSyntaxTree.ParseText(text);
        }
    }
}
