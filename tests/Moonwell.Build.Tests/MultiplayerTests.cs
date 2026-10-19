using Xunit;

namespace Moonwell.Build.Tests
{
    [Collection("Sample")]
    public sealed class MultiplayerTests
    {
        [Fact]
        public void multiplayer_mistakes_fail_the_build_at_their_lines()
        {
            var project = MapProject.Find(TestFolders.Fixture("Desyncs"));
            var log = new StringWriter();

            var error = Assert.Throws<BuildException>(() => DotnetBuild.Run(project, false, log));

            Assert.Equal("dotnet build failed", error.Message);
            var output = log.ToString();
            Assert.Matches(@"Mistakes\.cs\(12,\d+\): error MW0001: 'CreateUnit' creates a handle", output);
            Assert.Matches(@"Mistakes\.cs\(13,\d+\): error MW0002: 'GetCameraTargetPositionX'", output);
            Assert.Matches(@"Mistakes\.cs\(14,\d+\): error MW0003", output);
            Assert.Matches(@"Mistakes\.cs\(15,\d+\): error MW0004", output);
            Assert.Matches(@"Mistakes\.cs\(19,\d+\): error MW0005: 'long'", output);
            Assert.Matches(@"Mistakes\.cs\(20,\d+\): error MW0006: 'SelectGroupForPlayerBJ'", output);
        }
    }
}
