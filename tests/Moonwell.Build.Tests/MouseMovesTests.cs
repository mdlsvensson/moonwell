using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;

namespace Moonwell.Build.Tests
{
    /// <summary>Whether a map needs the mouse's move events, read from its built assembly: only a read of the synced position or a written OnMouseMoved does.</summary>
    public sealed class MouseMovesTests : IDisposable
    {
        private readonly TestFolders folders;

        public MouseMovesTests(ITestOutputHelper output)
        {
            folders = new TestFolders(output);
        }

        public void Dispose()
        {
            folders.Dispose();
        }

        [Theory]
        [InlineData("class A : IMouseWatch { public void OnMousePressed(Mouse mouse, MouseButton button) { } }")]
        [InlineData("class A { void F(Player player) { player.Mouse().ShowCursor(false); player.Mouse().SetScreenPosition(1, 2); } }")]
        [InlineData("class A { bool F(Player player) { return player.Mouse().IsDown(MouseButton.Left); } }")]
        [InlineData("class A { void F(Player player) { Local.For(player, () => { var x = player.Mouse().ScreenX; var u = player.Mouse().FocusUnit; var h = player.Mouse().IsHeldNow(MouseButton.Right); }); } }")]
        [InlineData("class A { void F(Player player) { player.Camera().Reset(); } }")]
        public void a_map_that_reads_no_position_needs_no_moves(string source)
        {
            Assert.False(MapBuild.NeedsMouseMoves(Compile(source)));
        }

        [Theory]
        [InlineData("class A { float F(Player player) { return player.Mouse().X; } }")]
        [InlineData("class A { float F(Player player) { return player.Mouse().Y; } }")]
        [InlineData("class A { Vector2 F(Player player) { return player.Mouse().Position; } }")]
        [InlineData("class A { bool F(Player player) { return player.Mouse().HasPosition; } }")]
        [InlineData("class A : IMouseWatch { public void OnMouseMoved(Mouse mouse) { } }")]
        [InlineData("class A : IMouseWatch { public void OnMousePressed(Mouse mouse, MouseButton button) { var p = mouse.Position; } }")]
        public void a_map_that_reads_the_position_or_writes_OnMouseMoved_needs_moves(string source)
        {
            Assert.True(MapBuild.NeedsMouseMoves(Compile(source)));
        }

        // Compiles a map's code against the built Moonwell.Input and writes the assembly into a folder outside the repository.
        private string Compile(string source)
        {
            var references = new List<MetadataReference>();
            foreach (var path in ((string)AppContext.GetData("TRUSTED_PLATFORM_ASSEMBLIES")!).Split(Path.PathSeparator))
            {
                if (!Path.GetFileName(path).StartsWith("Moonwell", StringComparison.Ordinal))
                {
                    references.Add(MetadataReference.CreateFromFile(path));
                }
            }

            foreach (var name in new[] { "Moonwell.dll", "Moonwell.Natives.dll", "Moonwell.Maths.dll", "Moonwell.Input.dll" })
            {
                references.Add(MetadataReference.CreateFromFile(Path.Combine(AppContext.BaseDirectory, name)));
            }

            var compilation = CSharpCompilation.Create(
                "MapCode",
                new[] { CSharpSyntaxTree.ParseText("using Moonwell;\n" + source) },
                references,
                new CSharpCompilationOptions(OutputKind.DynamicallyLinkedLibrary));
            var folder = folders.TemporaryFolderOutsideRepository();
            var assembly = Path.Combine(folder, "MapCode.dll");
            var result = compilation.Emit(assembly);
            Assert.True(result.Success, string.Join("\n", result.Diagnostics.Where(d => d.Severity == DiagnosticSeverity.Error)));
            return assembly;
        }
    }
}
