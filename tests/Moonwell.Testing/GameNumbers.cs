namespace Moonwell.Testing
{
    /// <summary>
    /// The numbers of the Lua the cases run in. .NET and the Lua test host have IEEE numbers; the game's own Lua has floats
    /// of its own (game-facts, 10-27): arithmetic makes no NaN or infinity (float.NaN, which CSharp.lua writes 0 / 0, is 1
    /// there), an addition drops what is below two float steps, and square roots and trig are off by up to about 2 parts
    /// in 100,000. The 0.3 gate played the cases in the game.
    /// </summary>
    public static class GameNumbers
    {
        /// <summary>Whether NaN exists where the cases run: false only in the game's own Lua, where a check given NaN checks nothing.</summary>
        public static bool HasNaN => float.IsNaN(float.NaN);
    }
}
