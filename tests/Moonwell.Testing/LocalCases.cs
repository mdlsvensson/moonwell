namespace Moonwell.Testing
{
    /// <summary>Cases that check Local.For.</summary>
    public static class LocalCases
    {
        /// <summary>The action runs when the player is the one at this machine.</summary>
        public static void RunsForLocalPlayer(FakeWorld world)
        {
            var ran = 0;
            Local.For(Player.Get(0), () => ran++);
            Expect.Equal(1, ran, "runs for the local player");
        }

        /// <summary>The action does not run for another player.</summary>
        public static void SkipsOtherPlayers(FakeWorld world)
        {
            var ran = 0;
            Local.For(Player.Get(1), () => ran++);
            Expect.Equal(0, ran, "runs for another player");
        }
    }
}
