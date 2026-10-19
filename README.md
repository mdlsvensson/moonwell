<div align="center">
  <img src="mw.gif" alt="Moonwell" width="256" />

  <p>
    <i>All-in-one modding framework for Warcraft III: Reforged.</i>
  </p>

  <p>
    <a href="https://www.hiveworkshop.com/">
      <img
        src="https://cdn.hiveworkshop.com/data/styles/7/styles/vindit-reforged/hive-logo.png"
        alt="Hive Workshop"
        height="36"
      />
    </a>
  </p>
</div>

**Moonwell** is a C# modding framework and build toolchain for [Warcraft III: Reforged](https://warcraft3.blizzard.com/).

A map is a C# project (`MyMap.csproj`) and a Warcraft III map saved as a folder (`map/`). You declare the map's settings, its units, heroes, items, abilities, buffs, destructables, doodads, and upgrades, all in code, and use the World Editor for terrain and placing things like regions, cameras, units, items, and trees. `moonwell build` writes declared object data into a staged copy of the map, compiles your C# into Lua, and packs the `.w3x` archive.
