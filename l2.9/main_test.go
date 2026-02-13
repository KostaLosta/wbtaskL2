package main

import "testing"

func TestOne(t *testing.T) {
	result, err := Unpack("a4bc2d5e")
	if err != nil {
		t.Fatal(err)
	}
	if result != "aaaabccddddde" {
		t.Errorf("error")
	}
}

func TestTwo(t *testing.T) {
	result, err := Unpack("abcd")
	if err != nil {
		t.Fatal(err)
	}
	if result != "abcd" {
		t.Errorf("error")
	}
}

func TestThree(t *testing.T) {
	result, err := Unpack("45")
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Errorf("error")
	}
}

func TestFour(t *testing.T) {
	result, err := Unpack("")
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Errorf("error")
	}
}

func TestFive(t *testing.T) {
	result, err := Unpack(`qwe\4\5`)
	if err != nil {
		t.Fatal(err)
	}
	if result != "qwe45" {
		t.Errorf("error")
	}
}

func TestSix(t *testing.T) {
	result, err := Unpack(`qwe\45`)
	if err != nil {
		t.Fatal(err)
	}
	if result != "qwe44444" {
		t.Errorf("error")
	}
}
