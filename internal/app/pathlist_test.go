package app

import "testing"

func TestPathList(t *testing.T) {
	const dir = `C:\Users\info\AppData\Local\grok-manager`
	if pathListHas("", dir) {
		t.Fatal("empty")
	}
	added := pathListAdd(`%USERPROFILE%\bin`, dir)
	if added != `%USERPROFILE%\bin;`+dir {
		t.Fatal("add")
	}
	if !pathListHas(added, dir+`\`) {
		t.Fatal("match slash")
	}
	again := pathListAdd(added, dir)
	if again != added {
		t.Fatal("duplicate")
	}
	if pathListRemove(added, `"`+dir+`"`) != `%USERPROFILE%\bin` {
		t.Fatal("remove")
	}
}
